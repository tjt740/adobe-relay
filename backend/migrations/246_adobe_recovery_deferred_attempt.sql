-- Zero reserves a cooldown after Okad explicitly declined to start a login.
-- Existing counts remain unchanged; two actual attempts remain the upper bound.
ALTER TABLE adobe_cookie_recovery_attempts
 DROP CONSTRAINT adobe_cookie_recovery_attempts_attempts_check;
ALTER TABLE adobe_cookie_recovery_attempts
 ADD CONSTRAINT adobe_cookie_recovery_attempts_attempts_check CHECK (attempts BETWEEN 0 AND 2);
