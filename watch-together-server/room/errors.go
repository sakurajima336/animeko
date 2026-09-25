package room

import "errors"

var (
	// ErrWrongPassword 房间密码错误。
	ErrWrongPassword = errors.New("WRONG_PASSWORD")
	// ErrRoomFull 房间已满。
	ErrRoomFull = errors.New("ROOM_FULL")
	// ErrRoomClosed 房间已关闭。
	ErrRoomClosed = errors.New("ROOM_CLOSED")
	// ErrInvalidName 房间名非法。
	ErrInvalidName = errors.New("INVALID_NAME")
	// ErrInvalidPassword 密码非法。
	ErrInvalidPassword = errors.New("INVALID_PASSWORD")
	// ErrNotMember 不是房间成员。
	ErrNotMember = errors.New("NOT_MEMBER")
)
