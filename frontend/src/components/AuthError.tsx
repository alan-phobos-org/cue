export function AuthError() {
  return (
    <div className="auth-error-page">
      <div className="auth-error-content">
        <h1>Authentication Required</h1>
        <p>This Cue instance requires a valid client certificate to access.</p>

        <h2>What you need</h2>
        <ul>
          <li>A client certificate signed by this server's Certificate Authority</li>
          <li>The certificate installed in your browser</li>
        </ul>

        <h2>Getting a certificate</h2>
        <p>
          Contact your system administrator to obtain a client certificate. They will
          need to sign it with the CA configured for this server.
        </p>

        <h2>Troubleshooting</h2>
        <ul>
          <li>
            <strong>Certificate not recognized?</strong> Ensure it's installed in your
            browser's certificate store
          </li>
          <li>
            <strong>Certificate expired?</strong> Request a new certificate from your
            administrator
          </li>
          <li>
            <strong>Using an API token?</strong> Tokens must be passed in the
            Authorization header, not via browser
          </li>
        </ul>
      </div>
    </div>
  );
}
