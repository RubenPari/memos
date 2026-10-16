package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/usememos/memos/core/memopayload"
	apipb "github.com/usememos/memos/proto/gen/api"
	"github.com/usememos/memos/store"
)

// RenameMemoTag renames a tag across the authenticated user's memos.
func (s *APIService) RenameMemoTag(ctx context.Context, request *apipb.RenameMemoTagRequest) (*emptypb.Empty, error) {
	user, err := s.fetchCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get current user")
	}
	if user == nil {
		return nil, status.Errorf(codes.Unauthenticated, "user not authenticated")
	}

	oldTag := strings.TrimSpace(request.OldTag)
	newTag := strings.TrimSpace(request.NewTag)
	if err := s.validateTagName(oldTag); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid old_tag: %v", err)
	}
	if err := s.validateTagName(newTag); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid new_tag: %v", err)
	}
	if oldTag == newTag {
		return nil, status.Errorf(codes.InvalidArgument, "new_tag must differ from old_tag")
	}

	memoFind := &store.FindMemo{
		CreatorID:       &user.ID,
		ExcludeComments: true,
		Filters:         []string{fmt.Sprintf("%s in tags", strconv.Quote(oldTag))},
	}
	if request.Parent != "memos/-" {
		memoUID, err := ExtractMemoUIDFromName(request.Parent)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid memo name: %v", err)
		}
		memo, err := s.Store.GetMemo(ctx, &store.FindMemo{UID: &memoUID})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get memo: %v", err)
		}
		if memo == nil || memo.CreatorID != user.ID {
			return nil, status.Errorf(codes.NotFound, "memo not found")
		}
		memoFind.UID = &memoUID
	}

	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list memos: %v", err)
	}

	renamed := false
	for _, memo := range memos {
		newContent, err := s.MarkdownService.RenameTag([]byte(memo.Content), oldTag, newTag)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to rename tag: %v", err)
		}
		if newContent == memo.Content {
			continue
		}
		memo.Content = newContent
		if err := memopayload.RebuildMemoPayload(ctx, memo, s.MarkdownService); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to rebuild memo payload: %v", err)
		}
		if err := s.Store.UpdateMemo(ctx, &store.UpdateMemo{
			ID:      memo.ID,
			Content: &memo.Content,
			Payload: memo.Payload,
			Policy:  memoWritePolicy(user.ID, false),
		}); err != nil {
			return nil, mapMemoWriteError(err, "failed to update memo")
		}
		renamed = true
	}

	if renamed && s.SSEHub != nil {
		s.SSEHub.publishMemoChanged()
	}
	return &emptypb.Empty{}, nil
}

// validateTagName reports whether name is exactly one tag as recognized by
// the markdown grammar, without the leading '#'.
func (s *APIService) validateTagName(name string) error {
	if name == "" {
		return fmt.Errorf("tag must not be empty")
	}
	tags, err := s.MarkdownService.ExtractTags([]byte("#" + name))
	if err != nil {
		return err
	}
	if len(tags) != 1 || tags[0] != name {
		return fmt.Errorf("%q is not a valid tag name", name)
	}
	return nil
}
