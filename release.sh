#!/bin/bash

# Check if a version argument is provided
if [ $# -eq 0 ]; then
    echo "Please provide a version number as an argument."
    exit 1
fi

# Get the version from the argument
version="$1"

# Get the current branch name
branch=$(git rev-parse --abbrev-ref HEAD)

# Check if the current branch is the main branch (e.g., master or main)
if [ "$branch" != "master" ] && [ "$branch" != "main" ]; then
    echo "Please switch to the main branch before creating an official release."
    exit 1
fi

# Update the version in the go.mod file
sed -i "s/version.*/version $version/" go.mod

# Commit the changes
git add go.mod
git commit -m "Release version $version"

# Create a new tag for the official release
tag_name="v$version"
git tag "$tag_name"

echo "Created official release: $tag_name"