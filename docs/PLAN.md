# Cue - Project Plan

## Vision

Cue is a personal/team knowledge management tool for storing and retrieving information on various topics. It provides a fast, searchable repository of items with markdown content and links to external resources.

**Target users**: Developers and technical teams who need quick access to documentation, notes, and bookmarks.

**Core value proposition**: Simple, fast, self-hosted knowledge base with full-text search and API-first design.

## Current Stage

**Version**: 0.2.0 (January 2026)

### Completed

- REST API backend (Go) with SQLite+FTS5 storage
- React frontend (TypeScript) with markdown rendering
- Items with titles, optional links (URL or file path), and markdown content
- Full-text search with BM25 ranking
- Single-user mode with TLS encryption
- Multi-user mode with mTLS client certificate authentication
- Token-based API authentication (HMAC-signed tokens)
- Version info via `/api/status`, health checks via `/api/health`
- Build system with lint, test, dist, and deploy targets
- PKI certificate generation for mTLS deployments
- Frontend embedded in Go binary for single-binary deployment

### Current Focus

Stabilization and documentation improvements. Preparing for wider usage.

## Backlog

### High Priority

- [ ] Token management UI in frontend (create, list, revoke tokens)
- [ ] User indicator in header showing current authenticated user
- [ ] Audit logging for security events (auth success/failure, token operations)

### Medium Priority

- [ ] CRL/OCSP certificate revocation checking
- [ ] Structured JSON logging for production deployments
- [ ] Request tracing with correlation IDs
- [ ] Token validation caching for high-traffic scenarios

### Low Priority / Future

- [ ] LDAP integration for user attributes and group lookup
- [ ] Per-item access control (optional, for team deployments)
- [ ] Browser extension for quick capture
- [ ] macOS native app
- [ ] Import/export functionality
- [ ] Tags/categories for items
- [ ] Item versioning/history

## Non-Goals

Things explicitly out of scope:

- **File uploads/attachments**: Items link to external resources only
- **Collaboration features**: No real-time editing, comments, or sharing
- **Complex authorization**: Simple user attribution, not fine-grained ACLs
- **Mobile app**: Web interface works on mobile; native apps not planned

## Related Documentation

- [DESIGN.md](DESIGN.md) - Architecture and technical design
- [AUTH_DESIGN.md](AUTH_DESIGN.md) - Authentication system design
