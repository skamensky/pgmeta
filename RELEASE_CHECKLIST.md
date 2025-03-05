# Release Checklist for pgmeta

This checklist helps ensure that all releases of pgmeta meet our quality standards. Complete all items before creating a new release tag.

## Code Quality Checks

- [ ] Run `go fmt ./...` to ensure all code is properly formatted
- [ ] Run `golangci-lint run ./...` to check for linting issues
- [ ] Fix any linting errors or warnings found

## Testing

- [ ] Run `go test ./internal/...` to verify all tests pass
- [ ] Run `go test -race ./internal/...` to check for race conditions
- [ ] Run `go test -coverprofile=coverage.txt -covermode=atomic ./internal/...` to check test coverage
- [ ] Ensure test coverage is maintained or improved

## Documentation

- [ ] Update README.md with any new features or changes
- [ ] Update command-line help text if new commands or flags were added
- [ ] Check that all exported functions have proper documentation comments

## Version Management

- [ ] Update version information in relevant files
- [ ] Choose an appropriate semantic version number (MAJOR.MINOR.PATCH)
  - MAJOR: Breaking changes
  - MINOR: New features, backward compatible
  - PATCH: Bug fixes, backward compatible

## Release Process

1. Complete all checks above
2. Commit all changes and push to main branch
3. Create a new tag with the version number:
   ```bash
   git tag -a v1.2.3 -m "Release v1.2.3: Brief description of changes"
   ```
4. Push the tag to GitHub:
   ```bash
   git push origin v1.2.3
   ```
5. Monitor the GitHub Actions workflow to ensure the release completes successfully
6. Verify the release on the GitHub Releases page

## Post-Release

- [ ] Test the released binaries on different platforms
- [ ] Update documentation website if applicable
- [ ] Announce the release in appropriate channels 