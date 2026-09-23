package crud

import (
	"reflect"
	"slices"

	"google.golang.org/protobuf/proto"
)

// ListResult 是不可变的查询结果，同时记录是否计算了总条数。
type ListResult[T any] struct {
	items         []T
	nextPageToken string
	totalSize     int64
	hasTotalSize  bool
}

// NewListResult 保证 include_total 与总条数是否已计算的状态一致。
func NewListResult[T any](
	query ListQuery,
	items []T,
	nextPageToken string,
	totalSize *int64,
) (ListResult[T], error) {
	if query.includeTotal && totalSize == nil {
		return ListResult[T]{}, internalError("total_size", "include_total requested but Count result is absent")
	}
	if !query.includeTotal && totalSize != nil {
		return ListResult[T]{}, internalError("total_size", "Count result is present when include_total is false")
	}
	result := ListResult[T]{
		items:         cloneListItems(items),
		nextPageToken: nextPageToken,
	}
	if totalSize != nil {
		if *totalSize < 0 {
			return ListResult[T]{}, internalError("total_size", "Count returned negative value %d", *totalSize)
		}
		result.totalSize = *totalSize
		result.hasTotalSize = true
	}
	return result, nil
}

// Items returns a copy of the page items and clones protobuf messages.
func (result ListResult[T]) Items() []T { return cloneListItems(result.items) }

// NextPageToken returns the opaque continuation token, or empty on the last page.
func (result ListResult[T]) NextPageToken() string { return result.nextPageToken }

// TotalSize 返回总条数以及是否已计算该值。
func (result ListResult[T]) TotalSize() (int64, bool) {
	return result.totalSize, result.hasTotalSize
}

func cloneListItems[T any](items []T) []T {
	cloned := slices.Clone(items)
	for index, item := range cloned {
		message, ok := any(item).(proto.Message)
		if !ok {
			continue
		}
		value := reflect.ValueOf(message)
		if value.Kind() == reflect.Pointer && value.IsNil() {
			continue
		}
		cloned[index] = any(proto.Clone(message)).(T)
	}
	return cloned
}
