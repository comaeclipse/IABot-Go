# Testing Guide for Phase 1 Improvements

## What Changed

### 1. Better Error Messages
**Before:** Links that failed showed "unknown"
**After:** Specific errors like "DNS lookup failed", "TLS/certificate error", "timeout", "connection refused", etc.

### 2. Better Status Classification
**Before:** All 2xx codes shown as raw status
**After:** 2xx shows as "OK", special handling for 403 Forbidden, 429 Rate Limited

### 3. Archive URL Detection
**Before:** Would try to check if archive URLs themselves are archived
**After:** Detects 20+ archive services and skips redundant checks

### 4. Better Wayback Results
**Before:** Would accept any snapshot, even with bad status codes
**After:** Only accepts 200/203/206 snapshots, validates timestamps (1996-2025)

## Testing Options

### Option 1: Deploy to Vercel (Recommended)

```bash
# Install Vercel CLI (if not already installed)
npm install -g vercel

# Deploy to preview environment
vercel

# Or deploy to production
vercel --prod
```

Then visit your deployment URL and test with the Blink-182 page.

### Option 2: Manual Testing Checklist

Visit your deployed app and scan the **Blink-182** Wikipedia page. Check the results table for:

#### Test Case 1: Live Links
- **URL:** `http://blink182.com`
- **Expected:** Should show "OK" instead of "200 OK"
- **Log:** Look for `[LIVE]` messages showing HEAD/GET attempts

#### Test Case 2: Dead Link with Archive
- **URL:** `http://www.altpress.com/features/entry/qa_mark_hoppus/`
- **Expected:** Should show "not archived" OR find the archive now with better detection
- **Log:** Look for `[WAYBACK] Raw API response` showing the full JSON

#### Test Case 3: DNS Failures
- **URL:** Any URL with invalid domain
- **Expected:** Should show "DNS lookup failed" instead of "unknown"
- **Log:** Look for error classification in `[LIVE]` messages

#### Test Case 4: Archive URLs
- **URL:** `https://web.archive.org/web/...` (if any in the page)
- **Expected:** Should show "archive URL (skipped)" in Live column
- **Expected:** Should show "is archive" in Wayback column
- **Log:** Look for `[SCAN] Detected as archive URL, skipping checks`

#### Test Case 5: Forbidden/Rate Limited
- **URL:** `http://blogs.villagevoice.com/...` (shows 403)
- **Expected:** Should show "403 Forbidden" instead of raw status
- **Log:** Look for classified status in `[LIVE]` messages

### Option 3: Check Logs in Vercel Dashboard

If you've already deployed:

1. Go to your Vercel dashboard
2. Select your project
3. Go to "Deployments" → Latest deployment
4. Click "Functions" tab
5. Find the `/api/index` function
6. Click to view logs

Look for the new log prefixes:
- `[SCAN]` - Overall scan progress
- `[LIVE]` - Live link checking
- `[WAYBACK]` - Wayback Machine queries

### Example Expected Log Output

```
[SCAN] Starting scan for page: Blink-182
[SCAN] Fetching links from MediaWiki API...
[SCAN] MediaWiki API response status: 200
[SCAN] Found 143 raw links from MediaWiki
[SCAN] Limiting to first 50 of 143 unique links
[SCAN] Processing 50 unique links
[SCAN] [1/50] Checking: http://blink182.com
[LIVE] HEAD response for http://blink182.com: 200 OK
[SCAN] [1/50] Live check: 200 OK
[WAYBACK] Checking http://blink182.com
[WAYBACK] Raw API response for http://blink182.com: {"url":"http://blink182.com","archived_snapshots":{"closest":{"status":"200","available":true,...}}}
[WAYBACK] Parsed response for http://blink182.com: Available=true, URL=http://web.archive.org/..., Status=200, Timestamp=20240101120000
[WAYBACK] Found archive for http://blink182.com: http://web.archive.org/... (status: 200)
[SCAN] [1/50] Wayback check: archived=true status=200
...
[SCAN] Completed scan: processed 50 links
```

## What to Look For

### ✅ Success Indicators
- [ ] Error messages are more specific (not just "unknown")
- [ ] 2xx status codes show as "OK"
- [ ] Archive URLs are detected and skipped
- [ ] Wayback responses include full JSON in logs
- [ ] Timestamps are validated (no pre-1996 or future dates)
- [ ] Progress shows X/Y links processed

### ❌ Issues to Watch For
- [ ] Scan still timing out prematurely (should run 5 minutes max)
- [ ] Archive URLs not being detected
- [ ] Wayback showing "not archived" for URLs that clearly have archives
- [ ] Any panics or crashes

## Known Issue We're Debugging

The `http://www.altpress.com/features/entry/qa_mark_hoppus/` URL:
- Our curl test shows it **IS** archived
- But your previous scan showed "not archived"
- New logging should reveal why (check `[WAYBACK] Raw API response` and `[WAYBACK] Parsed response` logs)

## Next Steps After Testing

Based on what you find:
1. If errors are still "unknown" → investigate error classification
2. If Wayback still wrong → check raw API responses in logs
3. If timeout issues → may need to adjust context timeout
4. If all good → move to Phase 2 or remaining Phase 1 items
