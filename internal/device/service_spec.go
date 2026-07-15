package device

import (
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type ServiceSpec struct {
	GOOS       string
	Name       string
	Executable string
	Root       string
	Endpoint   string
	Contents   string
}

func RenderServiceSpec(goos, root, executable string) (ServiceSpec, error) {
	if goos == "" {
		goos = runtime.GOOS
	}
	if root == "" || executable == "" || !filepath.IsAbs(root) || !filepath.IsAbs(executable) {
		return ServiceSpec{}, domain.NewError(domain.CodeInvalidArgument, "service paths must be absolute")
	}
	name := "multidesk-daemon"
	endpoint := filepath.Join(root, "daemon.sock")
	if goos != "windows" {
		if short, err := localEndpointPath(root); err == nil {
			endpoint = short
		}
	}
	spec := ServiceSpec{GOOS: goos, Name: name, Executable: executable, Root: root, Endpoint: endpoint}
	switch goos {
	case "darwin":
		spec.Contents = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
			"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n" +
			"<plist version=\"1.0\"><dict><key>Label</key><string>multidesk-daemon</string>" +
			"<key>ProgramArguments</key><array><string>" + xmlEscape(executable) + "</string><string>daemon</string><string>serve</string><string>--root</string><string>" + xmlEscape(root) + "</string></array>" +
			"<key>RunAtLoad</key><true/></dict></plist>\n"
	case "linux":
		spec.Contents = "[Unit]\nDescription=MultiAgentDesk Device Daemon\nAfter=default.target\n\n[Service]\nType=simple\nExecStart=" + shellEscape(executable) + " daemon serve --root " + shellEscape(root) + "\nRestart=on-failure\n\n[Install]\nWantedBy=default.target\n"
	case "windows":
		spec.Endpoint = `\\.\pipe\` + endpointName(root)
		spec.Contents = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Task xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\"><RegistrationInfo><URI>\\MultiAgentDesk\\multidesk-daemon</URI></RegistrationInfo><Principals><Principal id=\"Author\"><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals><Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><StartWhenAvailable>true</StartWhenAvailable></Settings><Actions Context=\"Author\"><Exec><Command>" + xmlEscape(executable) + "</Command><Arguments>daemon serve --root &quot;" + xmlEscape(root) + "&quot;</Arguments></Exec></Actions></Task>\n"
	default:
		return ServiceSpec{}, domain.NewError(domain.CodeUnsupportedPlatform, "service platform is unsupported")
	}
	return spec, nil
}

func shellEscape(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func xmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, "\"", "&quot;")
	return strings.ReplaceAll(value, "'", "&apos;")
}
