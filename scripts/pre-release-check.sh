#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Ensure cleanup happens even if script exits early
cleanup() {
    echo -e "\n${YELLOW}Cleaning up temporary files...${NC}"
    rm -f coverage.txt
}
trap cleanup EXIT

echo -e "${YELLOW}Running pre-release checks for pgmeta...${NC}"

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: go is not installed${NC}"
    exit 1
fi

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    echo -e "${RED}Error: golangci-lint is not installed${NC}"
    echo -e "${YELLOW}Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest${NC}"
    exit 1
fi

echo -e "\n${YELLOW}1. Running go fmt...${NC}"
go_fmt_output=$(go fmt ./...)
if [ -z "$go_fmt_output" ]; then
    echo -e "${GREEN}✓ Code is properly formatted${NC}"
else
    echo -e "${RED}✗ Code formatting issues found:${NC}"
    echo "$go_fmt_output"
    echo -e "${YELLOW}Files have been formatted. Please review and commit the changes.${NC}"
fi

echo -e "\n${YELLOW}2. Running golangci-lint...${NC}"
if golangci-lint run ./...; then
    echo -e "${GREEN}✓ No linting issues found${NC}"
else
    echo -e "${RED}✗ Linting issues found. Please fix them before releasing.${NC}"
    exit 1
fi

echo -e "\n${YELLOW}3. Running tests...${NC}"
if go test ./internal/...; then
    echo -e "${GREEN}✓ All tests passed${NC}"
else
    echo -e "${RED}✗ Some tests failed. Please fix them before releasing.${NC}"
    exit 1
fi

echo -e "\n${YELLOW}4. Running tests with race detection...${NC}"
if go test -race ./internal/...; then
    echo -e "${GREEN}✓ No race conditions detected${NC}"
else
    echo -e "${RED}✗ Race conditions detected. Please fix them before releasing.${NC}"
    exit 1
fi

echo -e "\n${YELLOW}5. Checking test coverage...${NC}"
go test -coverprofile=coverage.txt -covermode=atomic ./internal/...
coverage=$(go tool cover -func=coverage.txt | grep total | awk '{print $3}')
echo -e "${GREEN}✓ Test coverage: $coverage${NC}"

echo -e "\n${GREEN}All pre-release checks completed successfully!${NC}"
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Review the RELEASE_CHECKLIST.md file"
echo "2. Update version information if needed"
echo "3. Commit all changes"
echo "4. Create and push a new tag (e.g., git tag -a v1.0.0 -m \"Release v1.0.0\")"
echo "5. Push the tag (git push origin v1.0.0)"

# Cleanup is handled by the trap 