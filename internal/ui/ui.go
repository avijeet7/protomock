package ui

import (
	"html/template"
	"net/http"
	"sort"

	"github.com/avijeet7/protomock/internal/models"
)

const uiTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>ProtoMock Endpoints</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f4f4f4; color: #333; }
        h1 { color: #0056b3; }
        h2 { color: #0056b3; border-bottom: 2px solid #0056b3; padding-bottom: 5px; margin-top: 30px; }
        .container { background-color: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        ul { list-style-type: none; padding: 0; }
        li { background-color: #e9ecef; margin-bottom: 8px; padding: 10px; border-radius: 4px; display: flex; align-items: center; }
        li strong { min-width: 80px; display: inline-block; color: #007bff; }
        li span { margin-left: 10px; }
        .http-method { font-weight: bold; margin-right: 10px; padding: 3px 8px; border-radius: 3px; color: white; }
        .GET { background-color: #28a745; } /* Green */
        .POST { background-color: #007bff; } /* Blue */
        .PUT { background-color: #ffc107; } /* Yellow */
        .DELETE { background-color: #dc3545; } /* Red */
        .PATCH { background-color: #6f42c1; } /* Purple */
        .OTHER { background-color: #6c757d; } /* Gray */
        .proto-type { background-color: #17a2b8; color: white; padding: 2px 6px; border-radius: 3px; font-size: 0.8em; margin-left: 10px; }
        .json-type { background-color: #fd7e14; color: white; padding: 2px 6px; border-radius: 3px; font-size: 0.8em; margin-left: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>ProtoMock Endpoints Overview</h1>

        <h2>HTTP Endpoints</h2>
        {{ if .HTTPEndpoints }}
        <ul>
            {{ range .HTTPEndpoints }}
            <li>
                <span class="http-method {{ .Method }}">{{ .Method }}</span>
                <strong>URL:</strong> <span>{{ .URL }}</span>
                {{ if .IsProto }}<span class="proto-type">Protobuf</span>{{ else }}<span class="json-type">JSON</span>{{ end }}
            </li>
            {{ end }}
        </ul>
        {{ else }}
        <p>No HTTP endpoints registered.</p>
        {{ end }}

        <h2>gRPC Endpoints</h2>
        {{ if .GRPCEndpoints }}
        <ul>
            {{ range .GRPCEndpoints }}
            <li><strong>Method:</strong> <span>{{ . }}</span></li>
            {{ end }}
        </ul>
        {{ else }}
        <p>No gRPC endpoints registered.</p>
        {{ end }}
    </div>
</body>
</html>
`

type uiData struct {
	HTTPEndpoints []httpEndpoint
	GRPCEndpoints []string
}

type httpEndpoint struct {
	Method  string
	URL     string
	IsProto bool
}

// GenerateUI creates an http.HandlerFunc that serves a UI displaying registered endpoints.
func GenerateUI(httpRoutes []models.Route, grpcRoutes []models.Route) http.HandlerFunc {
	// Prepare HTTP endpoints data
	httpEndpointsMap := make(map[string]httpEndpoint)
	for _, route := range httpRoutes {
		key := route.Method + route.URL
		if _, exists := httpEndpointsMap[key]; !exists {
			httpEndpointsMap[key] = httpEndpoint{
				Method:  route.Method,
				URL:     route.URL,
				IsProto: route.ProtoEncoded,
			}
		}
	}

	var displayHTTPEndpoints []httpEndpoint
	for _, ep := range httpEndpointsMap {
		displayHTTPEndpoints = append(displayHTTPEndpoints, ep)
	}
	sort.Slice(displayHTTPEndpoints, func(i, j int) bool {
		if displayHTTPEndpoints[i].URL != displayHTTPEndpoints[j].URL {
			return displayHTTPEndpoints[i].URL < displayHTTPEndpoints[j].URL
		}
		return displayHTTPEndpoints[i].Method < displayHTTPEndpoints[j].Method
	})

	// Prepare gRPC endpoints data
	grpcEndpointsMap := make(map[string]struct{})
	for _, route := range grpcRoutes {
		grpcEndpointsMap[route.URL] = struct{}{}
	}

	var displayGRPCEndpoints []string
	for method := range grpcEndpointsMap {
		displayGRPCEndpoints = append(displayGRPCEndpoints, method)
	}
	sort.Strings(displayGRPCEndpoints)

	// Final data for the template
	data := uiData{
		HTTPEndpoints: displayHTTPEndpoints,
		GRPCEndpoints: displayGRPCEndpoints,
	}

	tmpl := template.Must(template.New("ui").Parse(uiTemplate))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err := tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to render UI: "+err.Error(), http.StatusInternalServerError)
		}
	}
}
