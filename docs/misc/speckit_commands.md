# Available Slash Commands

This document describes the available slash commands in Spec Kit, listed in the order they are typically used during the Spec-Driven Development process.

## Commands Overview

| Command | Description | Sample Prompt (Simple Task Manager Project) |
|---------|-------------|---------------------------------------------|
| `/speckit.constitution` | Create project principles and development guidelines that govern all subsequent development decisions | "Create principles focused on code quality, testing standards, and user experience consistency for a task management application" |
| `/speckit.specify` | Define functional requirements and user stories for what you want to build | "Develop a task manager app that allows users to create tasks, mark them as complete, set due dates, and organize tasks into categories. Users should be able to view all tasks, filter by status, and edit existing tasks." |
| (Optional)`/speckit.clarify` | Clarify underspecified areas in requirements before planning | "Clarify the task manager requirements: Should completed tasks be archived or deleted? What happens when a due date passes? Can tasks be assigned to other users?" |
| (Optional)`/speckit.checklist` | Generate custom quality checklists that validate requirements completeness, clarity, and consistency (like "unit tests for English") | "Generate a quality checklist to validate that all task manager requirements are complete, clear, and consistent" |
| `/speckit.plan` | Create technical implementation plans with your chosen tech stack | "Create a plan for the task manager using React with TypeScript, Node.js/Express backend, and SQLite database. Include REST API endpoints for CRUD operations on tasks." |
| `/speckit.tasks` | Generate actionable task lists for implementation based on the technical plan | "Break down the task manager implementation into specific, actionable tasks organized by user stories" |
| (Optional)`/speckit.analyze` | Cross-artifact consistency and coverage analysis (run after tasks, before implementation) | "Analyze the task manager specification, plan, and tasks for consistency and identify any gaps in coverage" |
| `/speckit.implement` | Execute all tasks to build the feature according to the plan | "Implement the task manager application following the generated tasks and technical plan" |

## Usage Flow

The typical workflow follows this sequence:

1. **Establish Foundation**: Use `/speckit.constitution` to create project principles
2. **Define Requirements**: Use `/speckit.specify` to document what to build
3. **Clarify Details**: Use `/speckit.clarify` to resolve ambiguities
4. **Validate Requirements**: Use `/speckit.checklist` to validate requirements quality
5. **Technical Planning**: Use `/speckit.plan` to determine how to build it
6. **Task Breakdown**: Use `/speckit.tasks` to create actionable steps
7. **Quality Check**: Use `/speckit.analyze` to ensure consistency
8. **Implementation**: Use `/speckit.implement` to build the solution


## Environment Variables

| Variable | Description |
|----------|-------------|
| `SPECIFY_FEATURE` | Override feature detection for non-Git repositories. Set to the feature directory name (e.g., `001-task-manager`) to work on a specific feature when not using Git branches. Must be set before using `/speckit.plan` or follow-up commands. |

## Notes

- Always run `/speckit.clarify` before `/speckit.plan` to reduce rework
- `/speckit.analyze` should be run after `/speckit.tasks` and before `/speckit.implement`
- The AI agent will execute local CLI commands during implementation - ensure required tools are installed
- Each command creates or updates files in the `.specify/` directory structure