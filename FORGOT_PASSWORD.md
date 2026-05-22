# Forgot Password Implementation

## Overview
This document describes the forgot password functionality implemented in the auth package. The implementation follows industry standards and security best practices.

## Architecture

### Components

1. **OTP Model** (`password_reset.go`)
   - `PasswordResetOTP`: Stores password reset OTP records
   - `ForgotPasswordRequest`: Request payload for initiating forgot password
   - `ResetPasswordRequest`: Request payload for resetting password with OTP
   - `ResetPasswordResponse`: Response payload for password reset

2. **Service Layer** (`password_reset_service.go`)
   - `CreatePasswordResetOTP()`: Generates and stores OTP with 10-minute expiry
   - `VerifyPasswordResetOTP()`: Verifies OTP and resets password in a transaction
   - `GetUnusedOTPByEmail()`: Retrieves an unused and non-expired OTP

3. **Utility Functions**
   - `util/otp.go`: OTP generation using cryptographically secure random
   - `util/email.go`: Email sending with goroutine support

4. **Controller** (`controller.go`)
   - `ForgotPassword()`: HTTP endpoint for initiating password reset
   - `ResetPassword()`: HTTP endpoint for completing password reset

5. **Routes** (`routes.go`)
   - `POST /api/user/forgot-password`: Initiate password reset
   - `POST /api/user/reset-password`: Complete password reset

## Key Features

### Security Features

1. **OTP Generation**
   - Uses cryptographically secure `crypto/rand` package
   - Generates 6-digit numeric OTP
   - Validates OTP format (6 digits only)

2. **Time-Based Expiry**
   - OTP valid for exactly 10 minutes
   - Automatic cleanup of expired OTPs
   - Server-side expiry validation

3. **One-Time Usage**
   - OTPs marked as used after verification
   - Used OTPs cannot be reused
   - Prevents replay attacks

4. **Transaction Safety**
   - Password update and OTP marking done atomically
   - Ensures consistency in case of failures

5. **Email Security**
   - Email sending in separate goroutine (non-blocking)
   - Error logging without exposing sensitive details
   - User existence validation (doesn't leak whether user exists)

### Database Schema

```sql
CREATE TABLE password_reset_otps (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    email VARCHAR(100) NOT NULL,
    otp VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    is_used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_password_reset_otps_user
        FOREIGN KEY (user_id) REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,
    
    CONSTRAINT chk_password_reset_otps_otp_length
        CHECK (LENGTH(otp) = 6 AND otp ~ '^\d+$')
);

-- Indexes for performance
CREATE INDEX idx_password_reset_otps_user_id ON password_reset_otps(user_id);
CREATE INDEX idx_password_reset_otps_email ON password_reset_otps(email);
CREATE INDEX idx_password_reset_otps_expires_at ON password_reset_otps(expires_at);
CREATE INDEX idx_password_reset_otps_email_unused ON password_reset_otps(email, is_used, expires_at);
```

## API Endpoints

### 1. Initiate Forgot Password
**POST** `/api/user/forgot-password`

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response (Success - 200):**
```json
{
  "message": "If an account with this email exists, a password reset OTP has been sent"
}
```

**Error Responses:**
- 400: Invalid request format
- 503: Authentication service unavailable

**Security Note:** Returns the same message regardless of whether the email exists (prevents user enumeration).

### 2. Reset Password with OTP
**POST** `/api/user/reset-password`

**Request:**
```json
{
  "email": "user@example.com",
  "otp": "123456",
  "password": "newPassword123"
}
```

**Response (Success - 200):**
```json
{
  "message": "Password reset successfully",
  "user": {
    "id": 1,
    "firstname": "John",
    "lastname": "Doe",
    "email": "user@example.com",
    "role": "User"
  }
}
```

**Error Responses:**
- 400: Invalid OTP (expired, already used, or incorrect)
- 503: Authentication service unavailable

## Flow Diagram

```
User Request
    ↓
[Forgot Password Endpoint]
    ↓
Verify user exists
    ↓
Generate 6-digit OTP
    ↓
Store OTP in database (expires in 10 min)
    ↓
Send email in goroutine
    ↓
Return success response immediately
    
---

User receives email with OTP
    ↓
User submits OTP and new password
    ↓
[Reset Password Endpoint]
    ↓
Validate OTP format
    ↓
Find OTP record
    ↓
Check OTP not expired
    ↓
Check OTP not already used
    ↓
Hash new password (bcrypt)
    ↓
[Transaction Start]
  - Update user password
  - Mark OTP as used
  - Cleanup old expired OTPs
[Transaction Commit]
    ↓
Return success response
```

## Email Template

The email sent to the user includes:
- User's first name
- 6-digit OTP
- Expiry time (10 minutes)
- Security warnings
- Support contact information

## Implementation Details

### OTP Generation
- Uses `crypto/rand` for cryptographic randomness
- Generates 3 random bytes and applies modulo to get 0-999999
- Pads with zeros to always return 6 digits

### Email Service
- Implements goroutine-based async sending
- Uses Gmail SMTP (configurable via environment variables)
- Logs errors without propagating to user
- Non-blocking: returns immediately to caller

### Error Handling
- Distinguishes between different error types
- Provides specific error messages for development
- Returns generic messages to API clients
- Logs all errors for debugging

## Environment Variables Required

```
GMAIL_USER=your-gmail@gmail.com
GMAIL_APP_PASSWORD=your-app-specific-password
```

## Testing

Unit tests are provided in:
- `password_reset_service_test.go`: Service layer tests
- `otp_test.go`: OTP generation tests

Run tests with:
```bash
go test ./internal/auth -v
go test ./internal/util -v
```

## Security Considerations

1. **Rate Limiting**: Should implement rate limiting to prevent brute force
2. **HTTPS Only**: Always use HTTPS in production
3. **Password Requirements**: Validate new password meets security requirements
4. **Audit Logging**: Consider logging all password reset attempts
5. **Cleanup**: Old OTPs are automatically cleaned up after 10 minutes
6. **Session Invalidation**: Consider invalidating other sessions after password reset

## Future Improvements

1. Add rate limiting per email/IP address
2. Add configurable OTP expiry time
3. Support SMS OTP delivery
4. Add backup codes for account recovery
5. Implement step-up authentication for sensitive operations
6. Add password reset token (alternative to OTP)
7. Implement email confirmation for password reset requests
