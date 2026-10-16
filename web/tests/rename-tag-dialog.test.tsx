import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RenameTagDialog from "@/components/RenameTagDialog";
import { memoServiceClient } from "@/connect";

vi.mock("@/utils/i18n", () => ({ useTranslate: () => (key: string) => key }));
vi.mock("@/connect", () => ({ memoServiceClient: { renameMemoTag: vi.fn() } }));
vi.mock("react-hot-toast", () => ({ toast: { success: vi.fn(), error: vi.fn() }, default: { success: vi.fn(), error: vi.fn() } }));

import { toast } from "react-hot-toast";

const renderDialog = (onSuccess = vi.fn()) => {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <RenameTagDialog open onOpenChange={vi.fn()} tag="work" onSuccess={onSuccess} />
    </QueryClientProvider>,
  );
  return onSuccess;
};

describe("RenameTagDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(memoServiceClient.renameMemoTag).mockResolvedValue({} as never);
  });

  it("renames the tag in all of the user's memos and reports the new name", async () => {
    const onSuccess = renderDialog();

    const input = screen.getByLabelText("tag.new-name");
    expect(input).toHaveValue("work");
    fireEvent.change(input, { target: { value: "done" } });
    fireEvent.click(screen.getByRole("button", { name: "common.confirm" }));

    await waitFor(() =>
      expect(memoServiceClient.renameMemoTag).toHaveBeenCalledWith({ parent: "memos/-", oldTag: "work", newTag: "done" }),
    );
    await waitFor(() => expect(onSuccess).toHaveBeenCalledWith("done"));
    expect(toast.success).toHaveBeenCalledWith("tag.rename-success");
  });

  it("rejects empty and unchanged names without calling the API", async () => {
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: "common.confirm" }));
    expect(toast.error).toHaveBeenCalledWith("tag.rename-error-repeat");

    const input = screen.getByLabelText("tag.new-name");
    fireEvent.change(input, { target: { value: "two words" } });
    fireEvent.click(screen.getByRole("button", { name: "common.confirm" }));
    expect(toast.error).toHaveBeenCalledWith("tag.rename-error-empty");

    expect(memoServiceClient.renameMemoTag).not.toHaveBeenCalled();
  });
});
