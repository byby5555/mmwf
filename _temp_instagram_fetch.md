---
type: concept
title: Instagram Reel Fetch Script
validate: false
ingested_at: '2026-07-02T07:15:00.456Z'
source_kind: 'mcp:put_page'
ingested_via: 'mcp:put_page'
---

```python
import urllib.request
import json
import re
import sys

url = "https://www.instagram.com/reel/DYTt0VJBo3S/"
headers = {
    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "en-US,en;q=0.5",
}

req = urllib.request.Request(url, headers=headers)
try:
    resp = urllib.request.urlopen(req, timeout=15)
    html = resp.read().decode('utf-8', errors='replace')
    print(f"Status: {resp.status}")
    print(f"Content-Length: {len(html)}")
    
    # Search for metadata in HTML
    # Look for og tags
    og_patterns = re.findall(r'<meta\s+property="([^"]+)"\s+content="([^"]*)"', html)
    for p in og_patterns:
        print(f"OG: {p[0]} = {p[1]}")
    
    # Look for JSON-LD
    jsonld = re.findall(r'<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>', html, re.DOTALL)
    for j in jsonld:
        try:
            data = json.loads(j)
            print(f"\nJSON-LD: {json.dumps(data, indent=2, ensure_ascii=False)[:2000]}")
        except:
            print(f"JSON-LD (raw, first 500): {j[:500]}")
    
    # Look for shortcode_media or any Instagram data
    shared_data = re.search(r'window\.__INITIAL_STATE__\s*=\s*({.*?});', html, re.DOTALL)
    if shared_data:
        print(f"\n__INITIAL_STATE__ found (len={len(shared_data.group(1))})")
        try:
            data = json.loads(shared_data.group(1))
            print(json.dumps(data, indent=2, ensure_ascii=False)[:3000])
        except:
            print(f"First 1000 chars: {shared_data.group(1)[:1000]}")
    else:
        print("\nNo __INITIAL_STATE__ found")
    
    # Look for any script with item data
    scripts = re.findall(r'<script[^>]*>(.*?)</script>', html, re.DOTALL)
    for i, s in enumerate(scripts):
        if 'reel' in s.lower() or 'video' in s.lower() or 'shortcode' in s.lower() or 'media' in s.lower():
            print(f"\nScript #{i} (len={len(s)}): {s[:500]}")
            if len(s) > 500:
                print(f"... (truncated, total {len(s)} chars)")
    
    # Look for description, title
    title = re.search(r'<title>(.*?)</title>', html, re.DOTALL)
    if title:
        print(f"\nTitle: {title.group(1)}")
    
    desc = re.search(r'<meta\s+name="description"\s+content="([^"]*)"', html)
    if desc:
        print(f"Description: {desc.group(1)}")
    
    print(f"\nTotal HTML size: {len(html)} bytes")
    
except Exception as e:
    print(f"Error: {e}")
    import traceback
    traceback.print_exc()
```
