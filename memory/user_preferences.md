# User Preferences

## Documentation Style
**Preference:** Comprehensive, detailed documentation with clear structure
**Why:** Enables better understanding of complex systems
**How to apply:** Always provide full context, architecture details, and examples when discussing code

## Memory Management
**Preference:** Structured memory organization in `memory/` directory
**Why:** Enables semantic search and context preservation across sessions
**How to apply:** Create separate files for different memory types (user, project, architecture, feedback)

## Code Quality
**Preference:** Clean, well-structured Go code with proper error handling
**Why:** Maintains codebase quality and maintainability
**How to apply:** Review code for clarity, proper abstractions, and error handling

## Git Workflow
**Preference:** Follow existing commit message conventions
**Why:** Maintains consistent history and tooling compatibility
**How to apply:** Use conventional commits format (`feat()`, `fix()`, `refactor()`, `docs()`)

## Debugging Approach
**Preference:** Systematic debugging with logs and diagnostics
**Why:** Enables efficient problem resolution
**How to apply:** Check logs, use MCP diagnostics, trace event flows

## MCP Tools Usage
**Preference:** Utilize available MCP tools (memory, IDE, LSP) when relevant
**Why:** Enhances productivity and code quality
**How to apply:** Use memory tools for context, LSP for code intelligence

## Project Structure
**Preference:** Follow established service organization pattern
**Why:** Maintains consistency across microservices
**How to apply:** Each service has README.md, configuration, and Docker support

## Configuration
**Preference:** Environment variables for configuration
**Why:** Enables deployment flexibility
**How to apply:** Use `env_file: .env` in docker-compose, document all required variables
