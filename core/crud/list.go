package crud

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"

	crudpb "github.com/Servora-Kit/servora/api/gen/go/servora/crud/v1"
	"google.golang.org/protobuf/proto"
)

const (
	defaultFilterMaxBytes   = 8 * 1024
	defaultFilterMaxNodes   = 128
	defaultFilterMaxDepth   = 8
	defaultFilterMaxORTerms = 64
	defaultOrderMaxBytes    = 2 * 1024
	defaultOrderMaxTerms    = 8
	defaultPageSize         = int32(50)
	defaultMaxPageSize      = int32(1000)
)

type limit struct {
	value   int
	limited bool
}

// FilterLimits contains immutable filter input and structure limits.
type FilterLimits struct {
	initialized bool
	maxBytes    limit
	maxNodes    limit
	maxDepth    limit
	maxORTerms  limit
}

// FilterLimitOption configures FilterLimits during construction.
type FilterLimitOption func(*FilterLimits) error

// NewFilterLimits returns filter limits initialized from framework defaults.
func NewFilterLimits(options ...FilterLimitOption) (FilterLimits, error) {
	limits := DefaultFilterLimits()
	for _, option := range options {
		if option == nil {
			return FilterLimits{}, fmt.Errorf("crud: filter limit option is nil")
		}
		if err := option(&limits); err != nil {
			return FilterLimits{}, err
		}
	}
	return limits, nil
}

// DefaultFilterLimits returns the framework filter limits.
func DefaultFilterLimits() FilterLimits {
	return FilterLimits{
		initialized: true,
		maxBytes:    limit{value: defaultFilterMaxBytes, limited: true},
		maxNodes:    limit{value: defaultFilterMaxNodes, limited: true},
		maxDepth:    limit{value: defaultFilterMaxDepth, limited: true},
		maxORTerms:  limit{value: defaultFilterMaxORTerms, limited: true},
	}
}

// UnlimitedFilterLimits explicitly disables every framework filter limit.
func UnlimitedFilterLimits() FilterLimits {
	return FilterLimits{initialized: true}
}

// MaxFilterBytes sets the maximum UTF-8 byte length of a filter.
func MaxFilterBytes(value int) FilterLimitOption {
	return setFilterLimit("max bytes", value, func(limits *FilterLimits, configured limit) {
		limits.maxBytes = configured
	})
}

// MaxFilterNodes sets the maximum parsed filter node count.
func MaxFilterNodes(value int) FilterLimitOption {
	return setFilterLimit("max nodes", value, func(limits *FilterLimits, configured limit) {
		limits.maxNodes = configured
	})
}

// MaxFilterDepth sets the maximum logical and parenthesis depth.
func MaxFilterDepth(value int) FilterLimitOption {
	return setFilterLimit("max depth", value, func(limits *FilterLimits, configured limit) {
		limits.maxDepth = configured
	})
}

// MaxFilterORTerms sets the maximum OR term count.
func MaxFilterORTerms(value int) FilterLimitOption {
	return setFilterLimit("max OR terms", value, func(limits *FilterLimits, configured limit) {
		limits.maxORTerms = configured
	})
}

// WithoutMaxFilterBytes explicitly disables the filter byte limit.
func WithoutMaxFilterBytes() FilterLimitOption {
	return clearFilterLimit(func(limits *FilterLimits) { limits.maxBytes = limit{} })
}

// WithoutMaxFilterNodes explicitly disables the filter node limit.
func WithoutMaxFilterNodes() FilterLimitOption {
	return clearFilterLimit(func(limits *FilterLimits) { limits.maxNodes = limit{} })
}

// WithoutMaxFilterDepth explicitly disables the filter depth limit.
func WithoutMaxFilterDepth() FilterLimitOption {
	return clearFilterLimit(func(limits *FilterLimits) { limits.maxDepth = limit{} })
}

// WithoutMaxFilterORTerms explicitly disables the filter OR term limit.
func WithoutMaxFilterORTerms() FilterLimitOption {
	return clearFilterLimit(func(limits *FilterLimits) { limits.maxORTerms = limit{} })
}

// MaxBytes returns the byte limit and whether it is enabled.
func (limits FilterLimits) MaxBytes() (int, bool) {
	return normalizeFilterLimits(limits).maxBytes.values()
}

// MaxNodes returns the node limit and whether it is enabled.
func (limits FilterLimits) MaxNodes() (int, bool) {
	return normalizeFilterLimits(limits).maxNodes.values()
}

// MaxDepth returns the depth limit and whether it is enabled.
func (limits FilterLimits) MaxDepth() (int, bool) {
	return normalizeFilterLimits(limits).maxDepth.values()
}

// MaxORTerms returns the OR term limit and whether it is enabled.
func (limits FilterLimits) MaxORTerms() (int, bool) {
	return normalizeFilterLimits(limits).maxORTerms.values()
}

// OrderLimits contains immutable order_by input limits.
type OrderLimits struct {
	initialized bool
	maxBytes    limit
	maxTerms    limit
}

// OrderLimitOption configures OrderLimits during construction.
type OrderLimitOption func(*OrderLimits) error

// NewOrderLimits returns order limits initialized from framework defaults.
func NewOrderLimits(options ...OrderLimitOption) (OrderLimits, error) {
	limits := DefaultOrderLimits()
	for _, option := range options {
		if option == nil {
			return OrderLimits{}, fmt.Errorf("crud: order limit option is nil")
		}
		if err := option(&limits); err != nil {
			return OrderLimits{}, err
		}
	}
	return limits, nil
}

// DefaultOrderLimits returns the framework order_by limits.
func DefaultOrderLimits() OrderLimits {
	return OrderLimits{
		initialized: true,
		maxBytes:    limit{value: defaultOrderMaxBytes, limited: true},
		maxTerms:    limit{value: defaultOrderMaxTerms, limited: true},
	}
}

// UnlimitedOrderLimits explicitly disables every framework order_by limit.
func UnlimitedOrderLimits() OrderLimits {
	return OrderLimits{initialized: true}
}

// MaxOrderBytes sets the maximum UTF-8 byte length of order_by.
func MaxOrderBytes(value int) OrderLimitOption {
	return setOrderLimit("max bytes", value, func(limits *OrderLimits, configured limit) {
		limits.maxBytes = configured
	})
}

// MaxOrderTerms sets the maximum client order term count.
func MaxOrderTerms(value int) OrderLimitOption {
	return setOrderLimit("max terms", value, func(limits *OrderLimits, configured limit) {
		limits.maxTerms = configured
	})
}

// WithoutMaxOrderBytes explicitly disables the order_by byte limit.
func WithoutMaxOrderBytes() OrderLimitOption {
	return clearOrderLimit(func(limits *OrderLimits) { limits.maxBytes = limit{} })
}

// WithoutMaxOrderTerms explicitly disables the order_by term limit.
func WithoutMaxOrderTerms() OrderLimitOption {
	return clearOrderLimit(func(limits *OrderLimits) { limits.maxTerms = limit{} })
}

// MaxBytes returns the byte limit and whether it is enabled.
func (limits OrderLimits) MaxBytes() (int, bool) {
	return normalizeOrderLimits(limits).maxBytes.values()
}

// MaxTerms returns the term limit and whether it is enabled.
func (limits OrderLimits) MaxTerms() (int, bool) {
	return normalizeOrderLimits(limits).maxTerms.values()
}

// ListSettings is an immutable resolved list configuration.
type ListSettings struct {
	filter             FilterLimits
	order              OrderLimits
	defaultPageSize    int32
	maxPageSize        int32
	maxPageSizeLimited bool
}

// FilterLimits returns the resolved filter limits.
func (settings ListSettings) FilterLimits() FilterLimits { return settings.filter }

// OrderLimits returns the resolved order_by limits.
func (settings ListSettings) OrderLimits() OrderLimits { return settings.order }

// DefaultPageSize returns the page size used when the request uses zero.
func (settings ListSettings) DefaultPageSize() int32 { return settings.defaultPageSize }

// MaxPageSize returns the page-size cap and whether it is enabled.
func (settings ListSettings) MaxPageSize() (int32, bool) {
	return settings.maxPageSize, settings.maxPageSizeLimited
}

// ListConfigOption modifies a ListSettings value during construction.
type ListConfigOption func(*ListSettings) error

// WithFilterLimits overrides the resolved filter limits.
func WithFilterLimits(limits FilterLimits) ListConfigOption {
	return func(settings *ListSettings) error {
		settings.filter = normalizeFilterLimits(limits)
		return nil
	}
}

// WithOrderLimits overrides the resolved order_by limits.
func WithOrderLimits(limits OrderLimits) ListConfigOption {
	return func(settings *ListSettings) error {
		settings.order = normalizeOrderLimits(limits)
		return nil
	}
}

// WithDefaultPageSize overrides the default page size.
func WithDefaultPageSize(value int32) ListConfigOption {
	return func(settings *ListSettings) error {
		if value <= 0 {
			return fmt.Errorf("crud: default page size must be positive, got %d", value)
		}
		settings.defaultPageSize = value
		return nil
	}
}

// WithMaxPageSize overrides the page-size cap.
func WithMaxPageSize(value int32) ListConfigOption {
	return func(settings *ListSettings) error {
		if value <= 0 {
			return fmt.Errorf("crud: maximum page size must be positive, got %d", value)
		}
		settings.maxPageSize = value
		settings.maxPageSizeLimited = true
		return nil
	}
}

// WithoutMaxPageSize explicitly disables the page-size cap.
func WithoutMaxPageSize() ListConfigOption {
	return func(settings *ListSettings) error {
		settings.maxPageSize = 0
		settings.maxPageSizeLimited = false
		return nil
	}
}

// ListPreparerOption configures a ListPreparer during construction.
type ListPreparerOption func(*listPreparerBuilder) error

// WithApplicationDefaults applies overrides above the framework defaults.
func WithApplicationDefaults(options ...ListConfigOption) ListPreparerOption {
	options = append([]ListConfigOption(nil), options...)
	return func(builder *listPreparerBuilder) error {
		builder.application = append(builder.application, options...)
		return nil
	}
}

// WithResourceOverrides applies the highest-priority overrides for a resource.
func WithResourceOverrides(resourceType string, options ...ListConfigOption) ListPreparerOption {
	options = append([]ListConfigOption(nil), options...)
	return func(builder *listPreparerBuilder) error {
		if strings.TrimSpace(resourceType) == "" {
			return fmt.Errorf("crud: resource override type is empty")
		}
		builder.resources[resourceType] = append(builder.resources[resourceType], options...)
		return nil
	}
}

type ListPreparer struct {
	application ListSettings
	resources   map[string]ListSettings
	codec       PageTokenCodec
}
type listPreparerBuilder struct {
	application []ListConfigOption
	resources   map[string][]ListConfigOption
	codec       PageTokenCodec
}

// WithPageTokenCodec replaces the default unsigned page-token transport.
func WithPageTokenCodec(codec PageTokenCodec) ListPreparerOption {
	return func(builder *listPreparerBuilder) error {
		if isNilInterface(codec) {
			return fmt.Errorf("crud: page token codec is nil")
		}
		builder.codec = codec
		return nil
	}
}

// NewListPreparer constructs an immutable preparer and validates every config.
func NewListPreparer(options ...ListPreparerOption) (*ListPreparer, error) {
	builder := listPreparerBuilder{
		resources: make(map[string][]ListConfigOption),
		codec:     NewUnsignedPageTokenCodec(),
	}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("crud: list preparer option is nil")
		}
		if err := option(&builder); err != nil {
			return nil, err
		}
	}

	application := frameworkListSettings()
	if err := applyListConfig(&application, builder.application); err != nil {
		return nil, fmt.Errorf("crud: application list defaults: %w", err)
	}
	resources := make(map[string]ListSettings, len(builder.resources))
	for resourceType, overrides := range builder.resources {
		settings := application
		if err := applyListConfig(&settings, overrides); err != nil {
			return nil, fmt.Errorf("crud: resource %s list overrides: %w", resourceType, err)
		}
		resources[resourceType] = settings
	}
	return &ListPreparer{application: application, resources: resources, codec: builder.codec}, nil
}

// SettingsFor resolves resource overrides over application and framework defaults.
func (preparer *ListPreparer) SettingsFor(resourceType string) ListSettings {
	if settings, ok := preparer.resources[resourceType]; ok {
		return settings
	}
	return preparer.application
}

// ListResource identifies one resource plan or generated query contract.
type ListResource interface {
	ResourceType() string
}

// ListInput contains standard List request fields before parsing.
type ListInput struct {
	Collection   string
	PageSize     int32
	PageToken    string
	Skip         int64
	Filter       string
	OrderBy      string
	IncludeTotal bool
}

type ListQuery struct {
	resourceType string
	collection   string
	pageSize     int32
	pageToken    *crudpb.PageTokenPayload
	skip         int64
	filter       FilterExpression
	orderBy      OrderExpression
	includeTotal bool
	codec        PageTokenCodec
}

// PrepareList applies resolved base pagination settings and parses a typed filter.
func (preparer *ListPreparer) PrepareList(resource ListResource, input ListInput) (ListQuery, error) {
	if isNilListResource(resource) {
		return ListQuery{}, fmt.Errorf("crud: list resource type is empty")
	}
	resourceType := resource.ResourceType()
	if strings.TrimSpace(resourceType) == "" {
		return ListQuery{}, fmt.Errorf("crud: list resource type is empty")
	}
	settings := preparer.SettingsFor(resourceType)
	pageSize := input.PageSize
	switch {
	case pageSize < 0:
		return ListQuery{}, invalidFieldValue("page_size", "must be non-negative")
	case pageSize == 0:
		pageSize = settings.defaultPageSize
	case settings.maxPageSizeLimited && pageSize > settings.maxPageSize:
		pageSize = settings.maxPageSize
	}
	if input.Skip < 0 {
		return ListQuery{}, invalidFieldValue("skip", "must be non-negative")
	}
	var filter FilterExpression
	var pageToken *crudpb.PageTokenPayload
	if input.PageToken != "" {
		decoded, err := preparer.codec.Decode(input.PageToken)
		if err != nil {
			return ListQuery{}, invalidPageToken("page_token", "%v", err)
		}
		if decoded == nil {
			return ListQuery{}, invalidPageToken("page_token", "codec returned a nil payload")
		}
		pageToken = proto.CloneOf(decoded)
	}
	filterLimits := normalizeFilterLimits(settings.filter)
	if maximum, limited := filterLimits.MaxBytes(); limited && len(input.Filter) > maximum {
		return ListQuery{}, invalidFilter("filter", "byte limit %d exceeded", maximum)
	}
	syntacticDepth, err := filterParenthesisDepth(input.Filter)
	if err != nil {
		return ListQuery{}, invalidFilter("filter", "%v", err)
	}
	if maximum, limited := filterLimits.MaxDepth(); limited && syntacticDepth > maximum {
		return ListQuery{}, invalidFilter("filter", "depth limit %d exceeded", maximum)
	}
	if strings.TrimSpace(input.Filter) != "" {
		filterResource, ok := resource.(filterResource)
		if !ok {
			return ListQuery{}, internalError("filter", "resource %q does not expose a descriptor plan", resourceType)
		}
		filter, err = parseFilter(input.Filter, filterResource, syntacticDepth)
		if err != nil {
			return ListQuery{}, invalidFilter("filter", "%v", err)
		}
		if maximum, limited := filterLimits.MaxNodes(); limited && filter.NodeCount() > maximum {
			return ListQuery{}, invalidFilter("filter", "node limit %d exceeded", maximum)
		}
		if maximum, limited := filterLimits.MaxDepth(); limited && filter.Depth() > maximum {
			return ListQuery{}, invalidFilter("filter", "depth limit %d exceeded", maximum)
		}
		if maximum, limited := filterLimits.MaxORTerms(); limited && filter.ORTerms() > maximum {
			return ListQuery{}, invalidFilter("filter", "OR term limit %d exceeded", maximum)
		}
	}
	var orderBy OrderExpression
	orderLimits := normalizeOrderLimits(settings.order)
	if maximum, limited := orderLimits.MaxBytes(); limited && len(input.OrderBy) > maximum {
		return ListQuery{}, invalidOrderBy("order_by", "byte limit %d exceeded", maximum)
	}
	if strings.TrimSpace(input.OrderBy) != "" {
		orderResource, ok := resource.(filterResource)
		if !ok {
			return ListQuery{}, internalError("order_by", "resource %q does not expose a descriptor plan", resourceType)
		}
		var err error
		orderBy, err = parseOrderBy(input.OrderBy, orderResource)
		if err != nil {
			return ListQuery{}, invalidOrderBy("order_by", "%v", err)
		}
		if maximum, limited := orderLimits.MaxTerms(); limited && orderBy.TermCount() > maximum {
			return ListQuery{}, invalidOrderBy("order_by", "term limit %d exceeded", maximum)
		}
	}
	return ListQuery{
		resourceType: resourceType,
		collection:   input.Collection,
		pageToken:    pageToken,
		pageSize:     pageSize,
		skip:         input.Skip,
		filter:       filter,
		orderBy:      orderBy,
		includeTotal: input.IncludeTotal,
		codec:        preparer.codec,
	}, nil
}

// ResourceType returns the resource type associated with the query.
func (query ListQuery) ResourceType() string { return query.resourceType }

// Collection returns the normalized collection target input.
func (query ListQuery) Collection() string { return query.collection }

// PageSize returns the resolved page size.
func (query ListQuery) PageSize() int32 { return query.pageSize }

// Skip returns the requested resource offset after any page token cursor.
func (query ListQuery) Skip() int64 { return query.skip }

// IncludeTotal reports whether Count was requested.
func (query ListQuery) IncludeTotal() bool { return query.includeTotal }

// PageTokenPayload returns a clone of the decoded payload, or nil when no token was supplied.
func (query ListQuery) PageTokenPayload() *crudpb.PageTokenPayload {
	return proto.CloneOf(query.pageToken)
}

// Filter returns the parsed backend-neutral filter.
func (query ListQuery) Filter() FilterExpression { return query.filter }

// OrderBy returns the parsed client order expression.
func (query ListQuery) OrderBy() OrderExpression { return query.orderBy }

// EncodePageToken serializes a continuation cursor with the same codec used by PrepareList.
func (query ListQuery) EncodePageToken(
	fingerprint [sha256.Size]byte,
	cursor []*crudpb.CursorValue,
) (string, error) {
	if isNilInterface(query.codec) {
		return "", internalError("page_token", "query codec is nil")
	}
	values := make([]*crudpb.CursorValue, len(cursor))
	for index, value := range cursor {
		if value == nil || value.GetValue() == nil {
			return "", internalError("page_token", "cursor[%d] is nil or unset", index)
		}
		values[index] = proto.CloneOf(value)
	}
	payload := &crudpb.PageTokenPayload{
		Version:            CurrentPageTokenVersion,
		ContextFingerprint: append([]byte(nil), fingerprint[:]...),
		Cursor:             values,
	}
	token, err := query.codec.Encode(payload)
	if err != nil {
		return "", internalError("page_token", "encode continuation token: %v", err)
	}
	if token == "" {
		return "", internalError("page_token", "codec returned an empty continuation token")
	}
	return token, nil
}

func isNilListResource(resource ListResource) bool { return isNilInterface(resource) }

func isNilInterface(input any) bool {
	if input == nil {
		return true
	}
	value := reflect.ValueOf(input)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func frameworkListSettings() ListSettings {
	return ListSettings{
		filter:             DefaultFilterLimits(),
		order:              DefaultOrderLimits(),
		defaultPageSize:    defaultPageSize,
		maxPageSize:        defaultMaxPageSize,
		maxPageSizeLimited: true,
	}
}

func applyListConfig(settings *ListSettings, options []ListConfigOption) error {
	for _, option := range options {
		if option == nil {
			return fmt.Errorf("list config option is nil")
		}
		if err := option(settings); err != nil {
			return err
		}
	}
	if settings.defaultPageSize <= 0 {
		return fmt.Errorf("default page size must be positive, got %d", settings.defaultPageSize)
	}
	if settings.maxPageSizeLimited && settings.defaultPageSize > settings.maxPageSize {
		return fmt.Errorf(
			"default page size %d exceeds maximum %d",
			settings.defaultPageSize,
			settings.maxPageSize,
		)
	}
	return nil
}

func normalizeFilterLimits(limits FilterLimits) FilterLimits {
	if !limits.initialized {
		return DefaultFilterLimits()
	}
	return limits
}

func normalizeOrderLimits(limits OrderLimits) OrderLimits {
	if !limits.initialized {
		return DefaultOrderLimits()
	}
	return limits
}

func setFilterLimit(
	name string,
	value int,
	set func(*FilterLimits, limit),
) FilterLimitOption {
	return func(limits *FilterLimits) error {
		if value <= 0 {
			return fmt.Errorf("crud: filter %s must be positive, got %d", name, value)
		}
		set(limits, limit{value: value, limited: true})
		return nil
	}
}

func clearFilterLimit(clear func(*FilterLimits)) FilterLimitOption {
	return func(limits *FilterLimits) error {
		clear(limits)
		return nil
	}
}

func setOrderLimit(
	name string,
	value int,
	set func(*OrderLimits, limit),
) OrderLimitOption {
	return func(limits *OrderLimits) error {
		if value <= 0 {
			return fmt.Errorf("crud: order %s must be positive, got %d", name, value)
		}
		set(limits, limit{value: value, limited: true})
		return nil
	}
}

func clearOrderLimit(clear func(*OrderLimits)) OrderLimitOption {
	return func(limits *OrderLimits) error {
		clear(limits)
		return nil
	}
}

func (configured limit) values() (int, bool) {
	return configured.value, configured.limited
}
