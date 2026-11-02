import json

with open('backend/docs/swagger.json', 'r', encoding='utf-8') as f:
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

with open('backend/docs/swagger.json', 'w', encoding='utf-8') as f:
    json.dump(sw, f, ensure_ascii=False, indent=4)

with open('backend/docs/docs.go', 'r', encoding='utf-8') as f:
    content = f.read()

start_marker = 'const docTemplate = `'
end_marker = '`\n\n// SwaggerInfo'
start_idx = content.find(start_marker)
end_idx = content.find(end_marker)

if start_idx != -1 and end_idx != -1:
    raw_json = json.dumps(sw, indent=4)
    new_content = content[:start_idx + len(start_marker)] + raw_json + content[end_idx:]
    with open('backend/docs/docs.go', 'w', encoding='utf-8') as f:
        f.write(new_content)
    print('Updated swagger.json and docs.go successfully!')
else:
    print('Markers not found! start:', start_idx, 'end:', end_idx)

