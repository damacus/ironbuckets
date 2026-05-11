🧹 [code health] Capture and log DataUsageInfo error in BucketsHandler

🎯 **What:** The code health issue addressed
The `ListBuckets` handler in `internal/handlers/buckets_handler.go` previously ignored the error returned from `mdm.DataUsageInfo`. This PR modifies the handler to capture this error and log it using `c.Logger().Warnf()` if it's not nil. It additionally runs formatting (`go fmt`) on the codebase.

💡 **Why:** How this improves maintainability
Ignoring errors can hide bugs or configuration issues (like lack of admin permissions to view data usage stats or unexpected connectivity failures with MinIO). Logging it at the `Warning` level ensures that we are aware of any failure retrieving this auxiliary data without blocking the main intent of listing user buckets.

✅ **Verification:** How you confirmed the change is safe
- Executed unit tests (`go test ./...`) successfully.
- Code changes were reviewed by static analyzer tools and `go fmt`.
- Reviewed the Echo Context's logging system to confirm proper warning propagation.
- Using standard `DataUsageInfo` with explicit nil checks doesn't change standard handler flow behavior on success or error.

✨ **Result:** The improvement achieved
A clear improvement to our logs indicating issues related to storage usage queries while preventing silent failures, enriching debugging and operability of IronBuckets.
