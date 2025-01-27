// Copyright (C) 2022 Specter Ops, Inc.
//
// This file is part of AzureHound.
//
// AzureHound is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// AzureHound is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/bloodhoundad/azurehound/client/query"
	"github.com/bloodhoundad/azurehound/client/rest"
	"github.com/bloodhoundad/azurehound/constants"
	"github.com/bloodhoundad/azurehound/models/azure"
)

func (s *azureClient) GetAzureADUser(ctx context.Context, objectId string, selectCols []string) (*azure.User, error) {
	var (
		path     = fmt.Sprintf("/%s/users/%s", constants.GraphApiVersion, objectId)
		params   = query.Params{Select: selectCols}.AsMap()
		response azure.UserList
	)
	if res, err := s.msgraph.Get(ctx, path, params, nil); err != nil {
		return nil, err
	} else if err := rest.Decode(res.Body, &response); err != nil {
		return nil, err
	} else {
		return &response.Value[0], nil
	}
}

func (s *azureClient) GetAzureADUsers(ctx context.Context, filter string, search string, orderBy string, selectCols []string, top int32, count bool) (azure.UserList, error) {
	var (
		path     = fmt.Sprintf("/%s/users", constants.GraphApiVersion)
		params   = query.Params{Filter: filter, Search: search, OrderBy: orderBy, Select: selectCols, Top: top, Count: count}
		headers  map[string]string
		response azure.UserList
	)

	count = count || search != "" || (filter != "" && orderBy != "") || strings.Contains(filter, "endsWith")
	if count {
		headers = make(map[string]string)
		headers["ConsistencyLevel"] = "eventual"
	}
	if res, err := s.msgraph.Get(ctx, path, params.AsMap(), headers); err != nil {
		return response, err
	} else if err := rest.Decode(res.Body, &response); err != nil {
		return response, err
	} else {
		return response, nil
	}
}

func (s *azureClient) ListAzureADUsers(ctx context.Context, filter string, search string, orderBy string, selectCols []string) <-chan azure.UserResult {
	out := make(chan azure.UserResult)

	go func() {
		defer close(out)

		var (
			errResult = azure.UserResult{}
			nextLink  string
		)

		if users, err := s.GetAzureADUsers(ctx, filter, search, orderBy, selectCols, 999, false); err != nil {
			errResult.Error = err
			out <- errResult
		} else {
			for _, u := range users.Value {
				out <- azure.UserResult{Ok: u}
			}

			nextLink = users.NextLink
			for nextLink != "" {
				var users azure.UserList
				if url, err := url.Parse(nextLink); err != nil {
					errResult.Error = err
					out <- errResult
					nextLink = ""
				} else if req, err := rest.NewRequest(ctx, "GET", url, nil, nil, nil); err != nil {
					errResult.Error = err
					out <- errResult
					nextLink = ""
				} else if res, err := s.msgraph.Send(req); err != nil {
					errResult.Error = err
					out <- errResult
					nextLink = ""
				} else if err := rest.Decode(res.Body, &users); err != nil {
					errResult.Error = err
					out <- errResult
					nextLink = ""
				} else {
					for _, u := range users.Value {
						out <- azure.UserResult{Ok: u}
					}
					nextLink = users.NextLink
				}
			}
		}
	}()
	return out
}
