---
name: "docker"
description: "Containerise the application for consistent, portable deployments."
whenToUse: "Use when the task requires this skill capability."
applyTo: "**"
---

# Skill: Docker Support

## Description
Containerise the application for consistent, portable deployments.

## Steps
1. Write a multi-stage Dockerfile – builder stage then minimal runtime image.
2. Pin base image versions (e.g. node:22-alpine).
3. Never run the container process as root.
4. Provide a docker-compose.yml for local development with all dependencies.
5. Add a .dockerignore to exclude node_modules, .git, secrets, etc.
