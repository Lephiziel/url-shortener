package grpcserver

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"url-shortener/internal/domain"
	"url-shortener/internal/service"
	shortenerpb "url-shortener/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	shortenerpb.UnimplementedShortenerServiceServer
	service service.Service
	baseURL string
}

func NewServer(s service.Service, baseURL string) *Server {
	return &Server{
		service: s,
		baseURL: baseURL,
	}
}

func (s *Server) CreateLink(ctx context.Context, in *shortenerpb.CreateLinkRequest) (*shortenerpb.CreateLinkReply, error) {
	req := in.GetUrl()

	res, err := s.service.CreateShortLink(ctx, req)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidURL) {
			return nil, status.Error(codes.InvalidArgument, "invalid url")
		}
		slog.Error("failed to create short link", "error", err)
		return nil, status.Error(codes.Internal, "something went wrong with creating url")
	}

	shortURL, urlErr := url.JoinPath(s.baseURL, res.Code)
	if urlErr != nil {
		slog.Error("something went wrong with joining baseurl and code", "error", urlErr)
		return nil, status.Error(codes.Internal, "something went wrong with creating url")
	}

	reply := shortenerpb.CreateLinkReply{
		ShortUrl: shortURL,
	}

	return &reply, nil
}

func (s *Server) GetLink(ctx context.Context, in *shortenerpb.GetLinkRequest) (*shortenerpb.GetLinkReply, error) {
	code := in.GetCode()

	if code == "" {
		return nil, status.Error(codes.InvalidArgument, "code is empty")
	}

	res, err := s.service.GetOriginalLink(ctx, code)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "url not found")
		}
		slog.Error("failed to get original url", "error", err)
		return nil, status.Error(codes.Internal, "something went wrong")
	}

	reply := shortenerpb.GetLinkReply{
		OriginalUrl: res.URL,
	}

	return &reply, nil
}
