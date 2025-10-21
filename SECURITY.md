# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

Only the latest version receives security updates. We recommend always using the most recent release.

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please follow these steps:

### Private Disclosure

**Do not** open a public GitHub issue for security vulnerabilities.

Instead, please report security vulnerabilities by emailing the maintainer directly or using GitHub's private vulnerability reporting feature:

1. Go to the [Security tab](https://github.com/zoobzio/shotel/security)
2. Click "Report a vulnerability"
3. Provide detailed information about the vulnerability

### What to Include

When reporting a vulnerability, please include:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact and severity assessment
- Suggested fix or mitigation (if known)
- Your contact information for follow-up

### Response Timeline

- **Initial Response**: Within 48 hours of report submission
- **Status Update**: Within 5 business days with assessment and timeline
- **Resolution**: Varies by severity, but critical issues will be prioritized

## Security Considerations

### OpenTelemetry Data

shotel transmits observability data (metrics, traces, logs) to OpenTelemetry collectors. Consider:

- **Sensitive Data**: Ensure that instrumented applications do not include sensitive information in span attributes, log messages, or metric labels
- **Network Security**: Use TLS for production OTLP endpoints (set `Insecure: false` in Config)
- **Authentication**: Configure OTLP collector authentication separately from shotel

### Dependencies

shotel depends on:
- OpenTelemetry SDK for Go
- metricz, tracez, hookz observability libraries

Security updates for these dependencies are monitored and applied promptly.

### Data Privacy

When using shotel:
- Review what data your application exposes through metrics, traces, and logs
- Implement appropriate filtering or sanitization for sensitive information
- Configure retention policies on your OTLP collector
- Follow your organization's data governance policies

## Security Best Practices

### Configuration

```go
// Use TLS in production
shotel.New(shotel.Config{
    ServiceName:     "my-service",
    Endpoint:        "collector.example.com:4317",
    MetricsInterval: 15 * time.Second,
    Insecure:        false, // Enable TLS
})
```

### Resource Limits

- Set appropriate `MetricsInterval` to avoid excessive polling
- Monitor memory usage with large numbers of metrics
- Implement graceful shutdown with proper context cancellation

### Access Control

- Restrict access to OTLP collector endpoints
- Use network policies to limit which services can send telemetry
- Implement authentication at the collector level

## Disclosure Policy

When a security vulnerability is confirmed:

1. A fix will be developed and tested privately
2. A security advisory will be published on GitHub
3. A new release will be tagged with the fix
4. Credit will be given to the reporter (unless anonymity is requested)

## Security Updates

Subscribe to security advisories:
- Watch the [shotel repository](https://github.com/zoobzio/shotel) for security updates
- Enable GitHub security alerts for your repositories using shotel

## Contact

For security-related questions or concerns, please open a private security advisory or contact the maintainers through GitHub.
