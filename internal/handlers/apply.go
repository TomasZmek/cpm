package handlers

import (
	"github.com/TomasZmek/cpm/internal/services"
	"github.com/gofiber/fiber/v3"
)

// applyFlash queues the flash message for the result of
// CaddyService.ApplyChange and reports whether the change was kept.
//
//   - accepted: success with successMsg
//   - Docker unreachable: the change is kept without validation, warning
//   - rejected: the previous configuration was restored, error with Caddy's reason
//   - rejected and the restore failed: error naming both problems
func applyFlash(c fiber.Ctx, result *services.ReloadResult, successMsg string) bool {
	switch {
	case result.Success:
		setFlash(c, "success", successMsg)
		return true
	case result.DockerUnreachable:
		setFlash(c, "warning", successMsg+" "+result.Error)
		return true
	default:
		setFlash(c, "error", applyErrorMessage(c, result))
		return false
	}
}

// applyErrorMessage explains a change that Caddy rejected.
func applyErrorMessage(c fiber.Ctx, result *services.ReloadResult) string {
	if result.RolledBack {
		return tl(c, "msg_change_rolled_back") + ": " + result.Error
	}
	return tl(c, "msg_change_rollback_failed") + ": " + result.Error + " (" + result.RollbackError + ")"
}
