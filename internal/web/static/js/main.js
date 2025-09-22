document.addEventListener('DOMContentLoaded', function () {
    fetch('/api/endpoints')
        .then(response => response.json())
        .then(data => {
            const httpEndpoints = document.getElementById('http-endpoints');
            data.http_endpoints.forEach(endpoint => {
                const li = document.createElement('li');

                let endpointHeader = `<div class="endpoint-header"><span class="http-method ${endpoint.method}">${endpoint.method}</span><span class="endpoint-url">${endpoint.url}</span>`;
                if (endpoint.is_proto) {
                    endpointHeader += `<span class="proto-type">Protobuf</span>`;
                } else {
                    endpointHeader += `<span class="json-type">JSON</span>`;
                }
                endpointHeader += `</div>`;

                let matchDetails = '';
                if (endpoint.header_match) {
                    matchDetails += `<div class="match-details"><h4>Matching Headers</h4><pre>${JSON.stringify(endpoint.header_match, null, 2)}</pre></div>`;
                }
                if (endpoint.body_match) {
                    matchDetails += `<div class="match-details"><h4>Matching Body</h4><pre>${JSON.stringify(endpoint.body_match, null, 2)}</pre></div>`;
                }

                li.innerHTML = endpointHeader + matchDetails;
                httpEndpoints.appendChild(li);
            });

            const grpcEndpoints = document.getElementById('grpc-endpoints');
            data.grpc_endpoints.forEach(endpoint => {
                const li = document.createElement('li');
                li.innerHTML = `<strong>Method:</strong> <span>${endpoint}</span>`;
                grpcEndpoints.appendChild(li);
            });
        });
});