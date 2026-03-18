package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(NewAuthnService, NewUserService, NewTestService, NewOrganizationService, NewApplicationService, NewTenantService, NewRbacService, NewPositionService, NewDictService)
