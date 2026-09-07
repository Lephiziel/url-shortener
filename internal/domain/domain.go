package domain

import (
	"errors"
	"time"
)

type Link struct {
	Code      string
	URL       string
	CreatedAt time.Time
}

var (
	ErrNotFound   = errors.New("the link was not found")
	ErrInvalidURL = errors.New("the url is not valid")
)
