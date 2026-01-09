package handlers

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// LogsPage renders the logs viewer page
func (h *Handler) LogsPage(c *fiber.Ctx) error {
	lines := c.QueryInt("lines", 100)

	logs, err := h.dockerService.GetLogs(lines)
	if err != nil {
		logs = []string{"Error fetching logs: " + err.Error()}
	}

	data := h.baseData(c, "Caddy Logs")
	data["Logs"] = logs
	data["Lines"] = lines
	data["LogCount"] = len(logs)
	data["Active"] = "logs"

	return c.Render("pages/logs", data, "layouts/base")
}

// HTMXLogsStream streams logs via Server-Sent Events
func (h *Handler) HTMXLogsStream(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("Transfer-Encoding", "chunked")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		lastLines := make(map[string]bool)

		for i := 0; i < 60; i++ { // Stream for 60 seconds max
			logs, err := h.dockerService.GetLogs(50)
			if err != nil {
				fmt.Fprintf(w, "data: Error: %s\n\n", err.Error())
				w.Flush()
				time.Sleep(5 * time.Second)
				continue
			}

			// Send only new lines
			for _, line := range logs {
				line = strings.TrimSpace(line)
				if line == "" || lastLines[line] {
					continue
				}
				lastLines[line] = true

				// Escape for SSE
				escaped := strings.ReplaceAll(line, "\n", "\\n")
				fmt.Fprintf(w, "data: %s\n\n", escaped)
			}

			w.Flush()
			time.Sleep(2 * time.Second)
		}
	})

	return nil
}
