-- First login plus two fallback retries. Preserve existing incident counts.
ALTER TABLE adobe_cookie_recovery_attempts
 DROP CONSTRAINT adobe_cookie_recovery_attempts_attempts_check;
ALTER TABLE adobe_cookie_recovery_attempts
 ADD CONSTRAINT adobe_cookie_recovery_attempts_attempts_check CHECK (attempts BETWEEN 0 AND 3);
