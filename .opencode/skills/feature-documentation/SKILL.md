---
name: "feature-documentation"
description: "Create and update feature specifications in the docs folder."
whenToUse: "Use when the request says create a feature, change feature, update feature documentation, or similar."
applyTo: "**"
---

# Skill: Feature Documentation

## Description
Create and maintain feature specifications as Markdown files in `docs/features/<category>/` or `docs/features/<category>/<sub-category>/`.

## Required Workflow
1. Confirm the request was understood before creating or changing the document.
2. The document should be free from implementation details.
3. Don't use specific software or technologies. Focus on the user-facing behavior and outcomes.
4. If the category is not provided, ask for it before creating the feature file.
5. If an optional sub-category may be needed and was not provided, ask whether one should be used.
6. Reuse an existing matching feature document when changing a feature instead of creating a duplicate.
7. Store the file in `docs/features/<category>/<feature-name>.feature.md` or `docs/features/<category>/<sub-category>/<feature-name>.feature.md`.
8. The file name should indicate the order in which the feature should be implemented. For example, `001-feature-name.feature.md` for the first feature, `002-next-feature.feature.md` for the second, and so on.
9. The file name should indicate which features can be implemented in parallel. For example, `001-A-feature-name.feature.md` and `001-B-another-feature.feature.md` can be implemented in parallel, while `002-next-feature.feature.md` should be implemented after both `001` features are completed.
10. When creating a new feature specification, follow the structure and tone of existing feature documents in the repository when available; otherwise, use the template in `templates/feature.template.md`.
11. Use a kebab-case filename and always end the file with `.feature.md`.
12. After creating or updating the feature document, present it to the user and allow them to review and modify the file before proceeding with any implementation.
13. After the user confirms the document is satisfactory, ask whether code implementation should proceed before writing any code.
14. Update the `docs/features/index.md` file to include the new feature.
15. Update the `docs/features/index.md` with a tree view of the implementation path, the critical path should be red. Use drawio to draw the tree view. Store it in `docs/features/assets/implementation-path.drawio.png` The link should be `![Implementation path](./assets/features/implementation-path.drawio.png)`

## Default Template

See [feature.template.md](./templates/feature.template.md)

## Trigger Phrases
- create a feature
- change feature
- update feature
- feature documentation
- feature doc
