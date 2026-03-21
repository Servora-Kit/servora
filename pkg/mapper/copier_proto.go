package mapper

import "github.com/jinzhu/copier"

// CopierMapper is a reflection-based bidirectional mapper between
// Proto/domain type P and Entity/storage type E.
// Type parameter order: P = proto/domain side, E = entity/storage side.
type CopierMapper[P any, E any] struct {
	converters   []copier.TypeConverter
	fieldMapping []copier.FieldNameMapping
	options      copier.Option
}

func NewCopierMapper[P any, E any]() *CopierMapper[P, E] {
	return &CopierMapper[P, E]{
		converters: make([]copier.TypeConverter, 0),
		options: copier.Option{
			IgnoreEmpty: false,
			DeepCopy:    true,
		},
	}
}

func (m *CopierMapper[P, E]) AppendConverter(c copier.TypeConverter) *CopierMapper[P, E] {
	m.converters = append(m.converters, c)
	return m
}

func (m *CopierMapper[P, E]) AppendConverters(cs []copier.TypeConverter) *CopierMapper[P, E] {
	m.converters = append(m.converters, cs...)
	return m
}

func (m *CopierMapper[P, E]) WithFieldMapping(mapping map[string]string) *CopierMapper[P, E] {
	for src, dst := range mapping {
		m.fieldMapping = append(m.fieldMapping,
			copier.FieldNameMapping{SrcType: new(E), DstType: new(P), Mapping: map[string]string{src: dst}},
			copier.FieldNameMapping{SrcType: new(P), DstType: new(E), Mapping: map[string]string{dst: src}},
		)
	}
	return m
}

func (m *CopierMapper[P, E]) buildOption() copier.Option {
	opt := m.options
	opt.Converters = m.converters
	if len(m.fieldMapping) > 0 {
		opt.FieldNameMapping = m.fieldMapping
	}
	return opt
}

// ToProto converts entity E to proto P. Returns (nil, nil) when input is nil.
func (m *CopierMapper[P, E]) ToProto(entity *E) (*P, error) {
	if entity == nil {
		return nil, nil
	}
	var p P
	if err := copier.CopyWithOption(&p, entity, m.buildOption()); err != nil {
		return nil, err
	}
	return &p, nil
}

// ToEntity converts proto P to entity E. Returns (nil, nil) when input is nil.
func (m *CopierMapper[P, E]) ToEntity(proto *P) (*E, error) {
	if proto == nil {
		return nil, nil
	}
	var e E
	if err := copier.CopyWithOption(&e, proto, m.buildOption()); err != nil {
		return nil, err
	}
	return &e, nil
}

// MustToProto converts entity to proto, panics on error.
func (m *CopierMapper[P, E]) MustToProto(entity *E) *P {
	p, err := m.ToProto(entity)
	if err != nil {
		panic("mapper: ToProto: " + err.Error())
	}
	return p
}

// MustToEntity converts proto to entity, panics on error.
func (m *CopierMapper[P, E]) MustToEntity(proto *P) *E {
	e, err := m.ToEntity(proto)
	if err != nil {
		panic("mapper: ToEntity: " + err.Error())
	}
	return e
}

func (m *CopierMapper[P, E]) ToProtoList(entities []*E) ([]*P, error) {
	if len(entities) == 0 {
		return nil, nil
	}
	result := make([]*P, 0, len(entities))
	for _, e := range entities {
		p, err := m.ToProto(e)
		if err != nil {
			return nil, err
		}
		if p != nil {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *CopierMapper[P, E]) ToEntityList(protos []*P) ([]*E, error) {
	if len(protos) == 0 {
		return nil, nil
	}
	result := make([]*E, 0, len(protos))
	for _, p := range protos {
		e, err := m.ToEntity(p)
		if err != nil {
			return nil, err
		}
		if e != nil {
			result = append(result, e)
		}
	}
	return result, nil
}
