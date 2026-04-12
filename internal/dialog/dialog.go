package dialog

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func Error(ctx context.Context, title, message string) {
	_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.ErrorDialog,
		Title:   title,
		Message: message,
	})
}
func Info(ctx context.Context, title, message string) {
	_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   title,
		Message: message,
	})
}
