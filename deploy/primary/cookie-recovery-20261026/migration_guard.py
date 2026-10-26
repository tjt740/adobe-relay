"""Use the production migration runner's TrimSpace checksum contract."""
import hashlib
NAME = '255_adobe_cookie_recovery_three_attempts.sql'
SQL = '-- First login plus two fallback retries. Preserve existing incident counts.\nALTER TABLE adobe_cookie_recovery_attempts\n DROP CONSTRAINT adobe_cookie_recovery_attempts_attempts_check;\nALTER TABLE adobe_cookie_recovery_attempts\n ADD CONSTRAINT adobe_cookie_recovery_attempts_attempts_check CHECK (attempts BETWEEN 0 AND 3);\n'

def migration_checksum(content):
    return hashlib.sha256(content.strip().encode()).hexdigest()

def validate_migrations(installed, before):
    assert NAME not in before, 'Migration was already present in the baseline'
    expected = dict(before)
    expected[NAME] = migration_checksum(SQL)
    assert installed == expected, 'Migration set or old checksum changed'
