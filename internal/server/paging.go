/*
Copyright 2026 Fahri Cap.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"time"

	"github.com/oreondev/harbor-resource-operator/pkg/harbor"
)

// maxPageSize is Harbor's and the tools' largest page.
const maxPageSize = 100

// PageArgs are the paging arguments of every list tool.
type PageArgs struct {
	Page     int `json:"page,omitempty" jsonschema:"page number, starting at 1 (default 1)"`
	PageSize int `json:"pageSize,omitempty" jsonschema:"items per page (default 25, at most 100)"`
}

// options normalises the paging arguments the way Harbor does.
func (p PageArgs) options() harbor.PageOptions {
	o := harbor.PageOptions{Page: max(p.Page, 1), PageSize: p.PageSize}
	if o.PageSize < 1 {
		o.PageSize = harbor.DefaultPageSize
	}
	o.PageSize = min(o.PageSize, maxPageSize)
	return o
}

// Page is one page of a list tool's results.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total" jsonschema:"number of matching items across all pages"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// pageOf wraps one page fetched from Harbor.
func pageOf[T any](items []T, total int64, o harbor.PageOptions) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Total: total, Page: o.Page, PageSize: o.PageSize}
}

// localPage returns page p of a list Harbor returned in full.
func localPage[T any](all []T, p PageArgs) Page[T] {
	o := p.options()
	start := min((o.Page-1)*o.PageSize, len(all))
	end := min(start+o.PageSize, len(all))
	return pageOf(all[start:end], int64(len(all)), o)
}

// mapAll converts every element with f.
func mapAll[T, U any](in []T, f func(T) U) []U {
	out := make([]U, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

// ts formats a Harbor time as RFC 3339, or "" for Harbor's zero time.
func ts(t time.Time) string {
	if t.IsZero() || t.Year() < 1971 {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
