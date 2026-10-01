package runtime

const (
	ImageNode       = "node:18-alpine"
	ImageGo         = "golang:1.24-alpine"
	ImagePython     = "python:3.12-alpine"
	ImageK6         = "grafana/k6:latest"
	ImagePlaywright = "mcr.microsoft.com/playwright:v1.49.1-jammy"
)

var Images = []string{
	ImageNode,
	ImageGo,
	ImagePython,
	ImageK6,
	ImagePlaywright,
}
