package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apipb "github.com/usememos/memos/proto/gen/api"
)

func TestRenameMemoTagAcrossOwnMemos(t *testing.T) {
	ctx := context.Background()
	ts := NewTestService(t)
	defer ts.Cleanup()

	owner, err := ts.CreateRegularUser(ctx, "rename-tag-owner")
	require.NoError(t, err)
	ownerCtx := ts.CreateUserContext(ctx, owner.ID)
	other, err := ts.CreateRegularUser(ctx, "rename-tag-other")
	require.NoError(t, err)
	otherCtx := ts.CreateUserContext(ctx, other.ID)

	first, err := ts.Service.CreateMemo(ownerCtx, &apipb.CreateMemoRequest{Memo: &apipb.Memo{
		Content:    "ship it #work and #work again, but not `#work` in code",
		Visibility: apipb.Visibility_PRIVATE,
	}})
	require.NoError(t, err)
	second, err := ts.Service.CreateMemo(ownerCtx, &apipb.CreateMemoRequest{Memo: &apipb.Memo{
		Content:    "unrelated #play",
		Visibility: apipb.Visibility_PRIVATE,
	}})
	require.NoError(t, err)
	otherMemo, err := ts.Service.CreateMemo(otherCtx, &apipb.CreateMemoRequest{Memo: &apipb.Memo{
		Content:    "someone else's #work",
		Visibility: apipb.Visibility_PRIVATE,
	}})
	require.NoError(t, err)

	_, err = ts.Service.RenameMemoTag(ownerCtx, &apipb.RenameMemoTagRequest{
		Parent: "memos/-",
		OldTag: "work",
		NewTag: "done",
	})
	require.NoError(t, err)

	renamed, err := ts.Service.GetMemo(ownerCtx, &apipb.GetMemoRequest{Name: first.Name})
	require.NoError(t, err)
	require.Equal(t, "ship it #done and #done again, but not `#work` in code", renamed.Content)
	require.Contains(t, renamed.Tags, "done")
	require.NotContains(t, renamed.Tags, "work")

	untouched, err := ts.Service.GetMemo(ownerCtx, &apipb.GetMemoRequest{Name: second.Name})
	require.NoError(t, err)
	require.Equal(t, "unrelated #play", untouched.Content)

	foreign, err := ts.Service.GetMemo(otherCtx, &apipb.GetMemoRequest{Name: otherMemo.Name})
	require.NoError(t, err)
	require.Equal(t, "someone else's #work", foreign.Content)
	require.Contains(t, foreign.Tags, "work")
}

func TestRenameMemoTagInSingleMemo(t *testing.T) {
	ctx := context.Background()
	ts := NewTestService(t)
	defer ts.Cleanup()

	owner, err := ts.CreateRegularUser(ctx, "rename-tag-single-owner")
	require.NoError(t, err)
	ownerCtx := ts.CreateUserContext(ctx, owner.ID)

	first, err := ts.Service.CreateMemo(ownerCtx, &apipb.CreateMemoRequest{Memo: &apipb.Memo{
		Content:    "one #work",
		Visibility: apipb.Visibility_PRIVATE,
	}})
	require.NoError(t, err)
	second, err := ts.Service.CreateMemo(ownerCtx, &apipb.CreateMemoRequest{Memo: &apipb.Memo{
		Content:    "two #work",
		Visibility: apipb.Visibility_PRIVATE,
	}})
	require.NoError(t, err)

	_, err = ts.Service.RenameMemoTag(ownerCtx, &apipb.RenameMemoTagRequest{
		Parent: first.Name,
		OldTag: "work",
		NewTag: "done",
	})
	require.NoError(t, err)

	renamed, err := ts.Service.GetMemo(ownerCtx, &apipb.GetMemoRequest{Name: first.Name})
	require.NoError(t, err)
	require.Equal(t, "one #done", renamed.Content)

	untouched, err := ts.Service.GetMemo(ownerCtx, &apipb.GetMemoRequest{Name: second.Name})
	require.NoError(t, err)
	require.Equal(t, "two #work", untouched.Content)
}

func TestRenameMemoTagValidationAndOwnership(t *testing.T) {
	ctx := context.Background()
	ts := NewTestService(t)
	defer ts.Cleanup()

	owner, err := ts.CreateRegularUser(ctx, "rename-tag-validation-owner")
	require.NoError(t, err)
	ownerCtx := ts.CreateUserContext(ctx, owner.ID)
	other, err := ts.CreateRegularUser(ctx, "rename-tag-validation-other")
	require.NoError(t, err)
	otherCtx := ts.CreateUserContext(ctx, other.ID)

	memo, err := ts.Service.CreateMemo(ownerCtx, &apipb.CreateMemoRequest{Memo: &apipb.Memo{
		Content:    "mine #work",
		Visibility: apipb.Visibility_PRIVATE,
	}})
	require.NoError(t, err)

	// Unauthenticated callers are rejected.
	_, err = ts.Service.RenameMemoTag(ctx, &apipb.RenameMemoTagRequest{Parent: "memos/-", OldTag: "work", NewTag: "done"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// Invalid names and no-op renames are rejected.
	for _, req := range []*apipb.RenameMemoTagRequest{
		{Parent: "memos/-", OldTag: "", NewTag: "done"},
		{Parent: "memos/-", OldTag: "work", NewTag: ""},
		{Parent: "memos/-", OldTag: "work", NewTag: "work"},
		{Parent: "memos/-", OldTag: "work", NewTag: "two words"},
		{Parent: "memos/-", OldTag: "not a tag", NewTag: "done"},
		{Parent: "not-a-memo", OldTag: "work", NewTag: "done"},
	} {
		_, err := ts.Service.RenameMemoTag(ownerCtx, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err), "request %+v", req)
	}

	// Renaming a tag that exists nowhere is a successful no-op.
	_, err = ts.Service.RenameMemoTag(ownerCtx, &apipb.RenameMemoTagRequest{Parent: "memos/-", OldTag: "missing", NewTag: "done"})
	require.NoError(t, err)

	// Another user cannot target the owner's memo directly.
	_, err = ts.Service.RenameMemoTag(otherCtx, &apipb.RenameMemoTagRequest{Parent: memo.Name, OldTag: "work", NewTag: "done"})
	require.Equal(t, codes.NotFound, status.Code(err))

	untouched, err := ts.Service.GetMemo(ownerCtx, &apipb.GetMemoRequest{Name: memo.Name})
	require.NoError(t, err)
	require.Equal(t, "mine #work", untouched.Content)
}
