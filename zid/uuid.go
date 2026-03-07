package zid

import "github.com/google/uuid"

type UUid struct {
	uuid uuid.UUID
}

func NewUuid() UUid {
	return UUid{
		uuid: uuid.New(),
	}
}

func (u UUid) GenId() string {
	return u.uuid.String()
}
