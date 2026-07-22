package mapper

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TypeConverter is a copier conversion contract accepted by WithConverters.
type TypeConverter = copier.TypeConverter

func builtinConverters() []copier.TypeConverter {
	return []copier.TypeConverter{
		{
			SrcType: time.Time{}, DstType: (*timestamppb.Timestamp)(nil),
			Fn: func(source any) (any, error) {
				value := timestamppb.New(source.(time.Time))
				if err := value.CheckValid(); err != nil {
					return nil, fmt.Errorf("invalid timestamp: %w", err)
				}
				return value, nil
			},
		},
		{
			SrcType: (*time.Time)(nil), DstType: (*timestamppb.Timestamp)(nil),
			Fn: func(source any) (any, error) {
				value := source.(*time.Time)
				if value == nil {
					return (*timestamppb.Timestamp)(nil), nil
				}
				result := timestamppb.New(*value)
				if err := result.CheckValid(); err != nil {
					return nil, fmt.Errorf("invalid timestamp: %w", err)
				}
				return result, nil
			},
		},
		{
			SrcType: "", DstType: (*string)(nil),
			Fn: func(source any) (any, error) {
				value := source.(string)
				return &value, nil
			},
		},
		{
			SrcType: (*string)(nil), DstType: "",
			Fn: func(source any) (any, error) {
				value := source.(*string)
				if value == nil {
					return "", nil
				}
				return *value, nil
			},
		},
		{
			SrcType: int64(0), DstType: (*int64)(nil),
			Fn: func(source any) (any, error) {
				value := source.(int64)
				return &value, nil
			},
		},
		{
			SrcType: uuid.UUID{}, DstType: "",
			Fn: func(source any) (any, error) { return source.(uuid.UUID).String(), nil },
		},
		{
			SrcType: int(0), DstType: int32(0),
			Fn: func(source any) (any, error) {
				value := source.(int)
				if int64(value) < math.MinInt32 || int64(value) > math.MaxInt32 {
					return nil, fmt.Errorf("int value %d is out of int32 range", value)
				}
				return int32(value), nil
			},
		},
	}
}
