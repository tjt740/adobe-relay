def validate_accounts_copy(seen, ended, total, active):
    assert seen and ended, 'Accounts COPY block was not completely read'
    assert 0 <= active <= total, 'Inconsistent parsed account counts'
