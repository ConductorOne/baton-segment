#!/bin/bash
# Script to finish containerizing baton-segment
# Run this from the baton-segment directory

set -e

echo "Building to verify compilation..."
go build -o /tmp/baton-segment-test ./cmd/baton-segment && echo "Build successful!"

echo "Staging all changes..."
git add -A

echo "Committing changes..."
git commit -F COMMIT_MSG.txt

echo "Pushing to remote..."
git push -u origin containerize-baton-segment

echo "Done! All changes have been committed and pushed."
echo "You can now create a pull request on GitHub."
