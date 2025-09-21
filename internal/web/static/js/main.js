document.addEventListener('DOMContentLoaded', function () {
    fetch('/api/endpoints')
        .then(response => response.json())
        .then(data => {
            const httpEndpoints = document.getElementById('http-endpoints');
            data.http_endpoints.forEach(endpoint => {
                const li = document.createElement('li');
                li.innerHTML = `<span class="http-method ${endpoint.method}">${endpoint.method}</span><strong>URL:</strong> <span>${endpoint.url}</span>`;
                if (endpoint.is_proto) {
                    li.innerHTML += `<span class="proto-type">Protobuf</span>`;
                } else {
                    li.innerHTML += `<span class="json-type">JSON</span>`;
                }
                li.addEventListener('click', () => {
                    document.getElementById('request-type').value = 'http';
                    document.getElementById('http-method').value = endpoint.method;
                    document.getElementById('url').value = endpoint.url;
                });
                httpEndpoints.appendChild(li);
            });

            const grpcEndpoints = document.getElementById('grpc-endpoints');
            data.grpc_endpoints.forEach(endpoint => {
                const li = document.createElement('li');
                li.innerHTML = `<strong>Method:</strong> <span>${endpoint}</span>`;
                li.addEventListener('click', () => {
                    document.getElementById('request-type').value = 'grpc';
                    document.getElementById('url').value = endpoint;
                });
                grpcEndpoints.appendChild(li);
            });
        });

    document.getElementById('request-form').addEventListener('submit', function (e) {
        e.preventDefault();
        const form = e.target;
        const data = new FormData(form);

        const fetchOptions = {
            method: data.get('http_method'),
            headers: JSON.parse(data.get('headers') || '{}'),
        };

        if (fetchOptions.method !== 'GET' && fetchOptions.method !== 'HEAD') {
            fetchOptions.body = data.get('body');
        }

        fetch(data.get('url'), fetchOptions)
            .then(response => {
                document.getElementById('response-content').textContent = `Status: ${response.status}`;
                return response.text();
            })
            .then(text => {
                try {
                    const json = JSON.parse(text);
                    document.getElementById('response-json-body').textContent = JSON.stringify(json, null, 2);
                } catch (e) {
                    document.getElementById('response-json-body').textContent = text;
                }
            })
            .catch(error => {
                document.getElementById('response-content').textContent = 'Error: ' + error;
            });
    });
});
