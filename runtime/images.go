package runtime

const (
	ImageNode = "node:24-alpine"
	// Go artifacts are prebuilt binaries, so no toolchain is needed. A glibc
	// image (not alpine/musl) is required or CGO-linked binaries fail with
	// "not found". Binaries built against a newer glibc than 2.39 won't run.
	ImageGo         = "ubuntu:24.04"
	ImagePython     = "python:3.12-alpine"
	ImageK6         = "grafana/k6:2.3.0"
	ImageLocust     = "locustio/locust:2.46.7"
	ImagePlaywright = "mcr.microsoft.com/playwright:v1.49.1-jammy"
)

var Images = []string{
	ImageNode,
	ImageGo,
	ImagePython,
	ImageK6,
	ImageLocust,
	ImagePlaywright,
}
