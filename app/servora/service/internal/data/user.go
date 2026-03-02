package data

import (
	"context"

	"entgo.io/ent/dialect/sql"

	"github.com/horonlee/servora/app/servora/service/internal/biz"
	"github.com/horonlee/servora/app/servora/service/internal/biz/entity"
	"github.com/horonlee/servora/app/servora/service/internal/data/ent"
	"github.com/horonlee/servora/app/servora/service/internal/data/ent/user"
	"github.com/horonlee/servora/pkg/helpers/hash"
	"github.com/horonlee/servora/pkg/logger"
	"github.com/horonlee/servora/pkg/mapper"
)

type userRepo struct {
	data   *Data
	log    *logger.Helper
	mapper *mapper.CopierMapper[entity.User, ent.User]
}

func NewUserRepo(data *Data, l logger.Logger) biz.UserRepo {
	return &userRepo{
		data:   data,
		log:    logger.NewHelper(l, logger.WithModule("user/data/servora-service")),
		mapper: mapper.New[entity.User, ent.User]().RegisterConverters(mapper.AllBuiltinConverters()),
	}
}

func (r *userRepo) SaveUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	if !hash.BcryptIsHashed(user.Password) {
		bcryptPassword, err := hash.BcryptHash(user.Password)
		if err != nil {
			return nil, err
		}
		user.Password = bcryptPassword
	}
	entUser := r.mapper.ToEntity(user)
	b := r.data.entClient.User.Create().
		SetName(entUser.Name).
		SetEmail(entUser.Email).
		SetPassword(entUser.Password).
		SetRole(entUser.Role)

	if entUser.ID > 0 {
		b.SetID(entUser.ID)
	}

	created, err := b.Save(ctx)
	if err != nil {
		r.log.Errorf("SaveUser failed: %v", err)
		return nil, err
	}
	return r.mapper.ToDomain(created), nil
}

func (r *userRepo) GetUserById(ctx context.Context, id int64) (*entity.User, error) {
	entUser, err := r.data.entClient.User.Query().Where(user.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapper.ToDomain(entUser), nil
}

func (r *userRepo) DeleteUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	err := r.data.entClient.User.DeleteOneID(user.ID).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepo) UpdateUser(ctx context.Context, user *entity.User) (*entity.User, error) {
	if !hash.BcryptIsHashed(user.Password) {
		bcryptPassword, err := hash.BcryptHash(user.Password)
		if err != nil {
			return nil, err
		}
		user.Password = bcryptPassword
	}
	entUser := r.mapper.ToEntity(user)
	updated, err := r.data.entClient.User.UpdateOneID(user.ID).
		SetName(entUser.Name).
		SetEmail(entUser.Email).
		SetPassword(entUser.Password).
		SetRole(entUser.Role).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.mapper.ToDomain(updated), nil
}

func (r *userRepo) ListUsers(ctx context.Context, page int32, pageSize int32) ([]*entity.User, int64, error) {
	offset := int((page - 1) * pageSize)
	limit := int(pageSize)

	query := r.data.entClient.User.Query().Order(user.ByID(sql.OrderDesc()))
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entUsers, err := query.Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, err
	}

	users := make([]*entity.User, 0, len(entUsers))
	for _, entUser := range entUsers {
		users = append(users, r.mapper.ToDomain(entUser))
	}

	return users, int64(total), nil
}
