import json
import os

script_dir = os.path.dirname(os.path.abspath(__file__))
backend_dir = os.path.abspath(os.path.join(script_dir, ".."))
swagger_path = os.path.join(backend_dir, "docs", "swagger.json")
docs_go_path = os.path.join(backend_dir, "docs", "docs.go")
openapi_path = os.path.join(backend_dir, "docs", "openapi.json")

with open(swagger_path, 'r', encoding='utf-8') as f:
    sw = json.load(f)

# Set top-level security
sw['security'] = [{'BearerAuth': []}]

# In paths, remove security from protected endpoints so they inherit, and set [] for public
for path, methods in sw.get('paths', {}).items():
    for method, op in methods.items():
        if path in ['/auth/login', '/auth/refresh', '/auth/logout']:
            op['security'] = []
        else:
            if 'security' in op:
                del op['security']

with open(swagger_path, 'w', encoding='utf-8') as f:
    json.dump(sw, f, ensure_ascii=False, indent=4)

with open(docs_go_path, 'r', encoding='utf-8') as f:
    content = f.read()

start_marker = 'const docTemplate = `'
end_marker = '`\n\n// SwaggerInfo'
start_idx = content.find(start_marker)
end_idx = content.find(end_marker)

if start_idx != -1 and end_idx != -1:
    raw_json = json.dumps(sw, indent=4)
    new_content = content[:start_idx + len(start_marker)] + raw_json + content[end_idx:]
    with open(docs_go_path, 'w', encoding='utf-8') as f:
        f.write(new_content)
    print('Updated swagger.json and docs.go successfully!')
else:
    print('Markers not found! start:', start_idx, 'end:', end_idx)

# Build OpenAPI 3.0.3
oas = {
    'openapi': '3.0.3',
    'info': sw['info'],
    'servers': [{'url': f'http://{sw.get("host", "localhost:8080")}{sw.get("basePath", "/api/v1")}'}],
    'security': [{'BearerAuth': []}],
    'paths': {},
    'components': {
        'securitySchemes': {
            'BearerAuth': {
                'type': 'http',
                'scheme': 'bearer',
                'bearerFormat': 'JWT',
                'description': 'Nhập JWT Access Token'
            }
        },
        'schemas': {}
    }
}

for k, v in sw.get('definitions', {}).items():
    s = json.dumps(v).replace('#/definitions/', '#/components/schemas/')
    oas['components']['schemas'][k] = json.loads(s)

for path, methods in sw.get('paths', {}).items():
    oas['paths'][path] = {}
    for method, op in methods.items():
        new_op = dict(op)
        s_op = json.dumps(new_op).replace('#/definitions/', '#/components/schemas/')
        new_op = json.loads(s_op)

        if path in ['/auth/login', '/auth/refresh', '/auth/logout']:
            new_op['security'] = []
        else:
            if 'security' in new_op:
                del new_op['security']

        params = new_op.get('parameters', [])
        new_params = []
        body_param = None
        for p in params:
            if p.get('in') == 'body':
                body_param = p
            else:
                new_params.append(p)
        if new_params:
            new_op['parameters'] = new_params
        elif 'parameters' in new_op:
            del new_op['parameters']

        if body_param:
            new_op['requestBody'] = {
                'description': body_param.get('description', ''),
                'required': body_param.get('required', True),
                'content': {
                    'application/json': {
                        'schema': body_param.get('schema', {})
                    }
                }
            }

        if 'responses' in new_op:
            for status, resp in new_op['responses'].items():
                if 'schema' in resp:
                    resp_schema = resp.pop('schema')
                    resp['content'] = {
                        'application/json': {
                            'schema': resp_schema
                        }
                    }

        oas['paths'][path][method] = new_op

with open(openapi_path, 'w', encoding='utf-8') as f:
    json.dump(oas, f, ensure_ascii=False, indent=2)

print('Updated openapi.json (OpenAPI 3.0.3) successfully!')


