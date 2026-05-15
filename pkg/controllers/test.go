resp, err := casClient.CreateCertificate(ctx, req)
if err == nil {
    return resp, primaryParent, nil
}

// 1. OBSERVABILITY: Log the primary failure if we have fallbacks to try
if len(issuerSpec.Fallbacks) > 0 {
    log := ctrl.LoggerFrom(ctx) // Assuming you are using controller-runtime based on our previous chat
    log.Info("Primary CA failed, attempting fallbacks", "error", err.Error())
} else {
    // Fail fast if no fallbacks are configured
    return nil, "", fmt.Errorf("casClient.CreateCertificate failed (no fallbacks configured): %w", err)
}

// Try each fallback in order
var allErrs []error
allErrs = append(allErrs, fmt.Errorf("primary attempt failed: %w", err))

for i, fb := range issuerSpec.Fallbacks {
    fbParent, fbBuildErr := buildFallbackParentString(fb)
    if fbBuildErr != nil {
        allErrs = append(allErrs, fmt.Errorf("fallback[%d] build parent error: %w", i, fbBuildErr))
        continue
    }

    // Update request for the fallback
    req.Parent = fbParent
    req.Certificate.CertificateTemplate = fb.CertificateTemplate
    req.IssuingCertificateAuthorityId = fb.CertificateAuthorityId
    req.RequestId = uuid.New().String()

    resp, fbCertErr := casClient.CreateCertificate(ctx, req)
    if fbCertErr != nil {
        allErrs = append(allErrs, fmt.Errorf("fallback[%d] (%s) failed: %w", i, fbParent, fbCertErr))
        continue
    }

    // 2. OBSERVABILITY: Log that we are operating in a degraded state
    log := ctrl.LoggerFrom(ctx)
    log.Info("Successfully created certificate using fallback CA", "fallbackIndex", i, "fallbackParent", fbParent)

    return resp, fbParent, nil
}

// 3. IDIOMATIC ERRORS: Use errors.Join to bundle everything together
return nil, "", fmt.Errorf("all CA pools failed: %w", errors.Join(allErrs...))