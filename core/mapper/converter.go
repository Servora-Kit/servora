package mapper

import (
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewTimeConverterPair 创建 time.Time 与 *time.Time 之间的转换器对
func NewTimeConverterPair() []copier.TypeConverter {
	return []copier.TypeConverter{
		// time.Time -> *time.Time
		{
			SrcType: time.Time{},
			DstType: (*time.Time)(nil),
			Fn: func(src any) (any, error) {
				t := src.(time.Time)
				if t.IsZero() {
					return nil, nil
				}
				return &t, nil
			},
		},
		// *time.Time -> time.Time
		{
			SrcType: (*time.Time)(nil),
			DstType: time.Time{},
			Fn: func(src any) (any, error) {
				if src == nil {
					return time.Time{}, nil
				}
				t := src.(*time.Time)
				if t == nil {
					return time.Time{}, nil
				}
				return *t, nil
			},
		},
	}
}

// NewTimestamppbConverterPair 创建 time.Time 与 *timestamppb.Timestamp 之间的转换器对
// 用于 protobuf 时间类型转换
func NewTimestamppbConverterPair() []copier.TypeConverter {
	return []copier.TypeConverter{
		// time.Time -> *timestamppb.Timestamp
		{
			SrcType: time.Time{},
			DstType: (*timestamppb.Timestamp)(nil),
			Fn: func(src any) (any, error) {
				t := src.(time.Time)
				if t.IsZero() {
					return nil, nil
				}
				return timestamppb.New(t), nil
			},
		},
		// *timestamppb.Timestamp -> time.Time
		{
			SrcType: (*timestamppb.Timestamp)(nil),
			DstType: time.Time{},
			Fn: func(src any) (any, error) {
				if src == nil {
					return time.Time{}, nil
				}
				t := src.(*timestamppb.Timestamp)
				if t == nil {
					return time.Time{}, nil
				}
				return t.AsTime(), nil
			},
		},
	}
}

// NewStringPointerConverterPair 创建 string 与 *string 之间的转换器对
func NewStringPointerConverterPair() []copier.TypeConverter {
	return []copier.TypeConverter{
		// string -> *string
		{
			SrcType: "",
			DstType: (*string)(nil),
			Fn: func(src any) (any, error) {
				s := src.(string)
				if s == "" {
					return nil, nil
				}
				return &s, nil
			},
		},
		// *string -> string
		{
			SrcType: (*string)(nil),
			DstType: "",
			Fn: func(src any) (any, error) {
				if src == nil {
					return "", nil
				}
				s := src.(*string)
				if s == nil {
					return "", nil
				}
				return *s, nil
			},
		},
	}
}

// NewInt64PointerConverterPair 创建 int64 与 *int64 之间的转换器对
func NewInt64PointerConverterPair() []copier.TypeConverter {
	return []copier.TypeConverter{
		// int64 -> *int64
		{
			SrcType: int64(0),
			DstType: (*int64)(nil),
			Fn: func(src any) (any, error) {
				i := src.(int64)
				return &i, nil
			},
		},
		// *int64 -> int64
		{
			SrcType: (*int64)(nil),
			DstType: int64(0),
			Fn: func(src any) (any, error) {
				if src == nil {
					return int64(0), nil
				}
				i := src.(*int64)
				if i == nil {
					return int64(0), nil
				}
				return *i, nil
			},
		},
	}
}

// EnumConverter 枚举类型转换器
// DTO 通常是 int32 (protobuf enum)
// Entity 通常是 string (数据库存储)
type EnumConverter[DTO ~int32, Entity ~string] struct {
	nameMap  map[int32]string // enum value -> string name
	valueMap map[string]int32 // string name -> enum value
}

// NewEnumConverter 创建枚举转换器
// nameMap: protobuf 生成的 XXX_name map
// valueMap: protobuf 生成的 XXX_value map
func NewEnumConverter[DTO ~int32, Entity ~string](
	nameMap map[int32]string,
	valueMap map[string]int32,
) *EnumConverter[DTO, Entity] {
	return &EnumConverter[DTO, Entity]{
		nameMap:  nameMap,
		valueMap: valueMap,
	}
}

// ToDomain 将 Entity 字符串转换为 DTO 枚举值
func (c *EnumConverter[DTO, Entity]) ToDomain(entity *Entity) *DTO {
	if entity == nil {
		return nil
	}
	if val, ok := c.valueMap[string(*entity)]; ok {
		dto := DTO(val)
		return &dto
	}
	return nil
}

// ToEntity 将 DTO 枚举值转换为 Entity 字符串
func (c *EnumConverter[DTO, Entity]) ToEntity(dto *DTO) *Entity {
	if dto == nil {
		return nil
	}
	if name, ok := c.nameMap[int32(*dto)]; ok {
		entity := Entity(name)
		return &entity
	}
	return nil
}

// NewConverterPair 生成 copier.TypeConverter 对
func (c *EnumConverter[DTO, Entity]) NewConverterPair() []copier.TypeConverter {
	var dtoZero DTO
	var entityZero Entity

	return []copier.TypeConverter{
		// Entity -> DTO
		{
			SrcType: entityZero,
			DstType: dtoZero,
			Fn: func(src any) (any, error) {
				entity := src.(Entity)
				if val, ok := c.valueMap[string(entity)]; ok {
					return DTO(val), nil
				}
				return dtoZero, nil
			},
		},
		// DTO -> Entity
		{
			SrcType: dtoZero,
			DstType: entityZero,
			Fn: func(src any) (any, error) {
				dto := src.(DTO)
				if name, ok := c.nameMap[int32(dto)]; ok {
					return Entity(name), nil
				}
				return entityZero, nil
			},
		},
	}
}

// NewGenericConverterPair 创建任意类型之间的自定义转换器对
func NewGenericConverterPair[A any, B any](
	aToB func(A) (B, error),
	bToA func(B) (A, error),
) []copier.TypeConverter {
	var aZero A
	var bZero B

	return []copier.TypeConverter{
		{
			SrcType: aZero,
			DstType: bZero,
			Fn: func(src any) (any, error) {
				a := src.(A)
				return aToB(a)
			},
		},
		{
			SrcType: bZero,
			DstType: aZero,
			Fn: func(src any) (any, error) {
				b := src.(B)
				return bToA(b)
			},
		},
	}
}

// NewUUIDStringConverterPair creates converters between uuid.UUID and string.
// Covers the common case where ent uses uuid.UUID for primary keys and proto uses string.
func NewUUIDStringConverterPair() []copier.TypeConverter {
	return NewGenericConverterPair(
		func(id uuid.UUID) (string, error) { return id.String(), nil },
		func(s string) (uuid.UUID, error) { return uuid.Parse(s) },
	)
}

// NewIntInt32ConverterPair creates converters between int and int32.
// Covers the common case where ent uses int and proto uses int32.
func NewIntInt32ConverterPair() []copier.TypeConverter {
	return NewGenericConverterPair(
		func(i int) (int32, error) { return int32(i), nil },
		func(i int32) (int, error) { return int(i), nil },
	)
}

// AllBuiltinConverters 返回所有内置转换器
func AllBuiltinConverters() []copier.TypeConverter {
	converters := make([]copier.TypeConverter, 0)
	converters = append(converters, NewTimeConverterPair()...)
	converters = append(converters, NewTimestamppbConverterPair()...)
	converters = append(converters, NewStringPointerConverterPair()...)
	converters = append(converters, NewInt64PointerConverterPair()...)
	converters = append(converters, NewUUIDStringConverterPair()...)
	converters = append(converters, NewIntInt32ConverterPair()...)
	return converters
}

// Ptr 返回值的指针（辅助函数）
func Ptr[T any](v T) *T {
	return &v
}

// Val 返回指针的值，如果为 nil 则返回零值（辅助函数）
func Val[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// IsNil 检查 any 是否为 nil（包括值为 nil 的指针）
func IsNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return rv.IsNil()
	}
	return false
}
