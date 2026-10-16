import { useQueryClient } from "@tanstack/react-query";
import React, { useEffect, useState } from "react";
import { toast } from "react-hot-toast";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { memoServiceClient } from "@/connect";
import useLoading from "@/hooks/useLoading";
import { memoKeys } from "@/hooks/useMemoQueries";
import { userKeys } from "@/hooks/useUserQueries";
import { handleError } from "@/lib/error";
import { useTranslate } from "@/utils/i18n";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tag: string;
  onSuccess?: (newTag: string) => void;
}

function RenameTagDialog({ open, onOpenChange, tag, onSuccess }: Props) {
  const t = useTranslate();
  const queryClient = useQueryClient();
  const [newName, setNewName] = useState(tag);
  const requestState = useLoading(false);

  useEffect(() => {
    if (open) setNewName(tag);
  }, [open, tag]);

  const handleConfirm = async () => {
    const name = newName.trim();
    if (!name || /\s/.test(name)) {
      toast.error(t("tag.rename-error-empty"));
      return;
    }
    if (name === tag) {
      toast.error(t("tag.rename-error-repeat"));
      return;
    }

    try {
      requestState.setLoading();
      await memoServiceClient.renameMemoTag({
        parent: "memos/-",
        oldTag: tag,
        newTag: name,
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: memoKeys.lists() }),
        queryClient.invalidateQueries({ queryKey: userKeys.stats() }),
      ]);
      toast.success(t("tag.rename-success"));
      requestState.setFinish();
      onSuccess?.(name);
      onOpenChange(false);
    } catch (error: unknown) {
      handleError(error, toast.error, {
        context: "Rename tag",
        onError: () => requestState.setError(),
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t("tag.rename-tag")}</DialogTitle>
        </DialogHeader>
        <div className="grid gap-2">
          <Label htmlFor="rename-tag-name">{t("tag.new-name")}</Label>
          <Input
            id="rename-tag-name"
            type="text"
            value={newName}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewName(e.target.value)}
            onKeyDown={(e: React.KeyboardEvent<HTMLInputElement>) => {
              if (e.key === "Enter") void handleConfirm();
            }}
          />
          <p className="text-xs text-muted-foreground">{t("tag.rename-merge-hint")}</p>
        </div>
        <DialogFooter>
          <Button variant="ghost" disabled={requestState.isLoading} onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button disabled={requestState.isLoading} onClick={() => void handleConfirm()}>
            {t("common.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default RenameTagDialog;
