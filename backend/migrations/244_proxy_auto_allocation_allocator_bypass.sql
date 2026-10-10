-- Keep the allocator's atomic reconciliation writes from being re-paused by
-- the account trigger. The all-account trigger introduced in migration 243
-- must retain the transaction-local bypass used by the worker.
CREATE OR REPLACE FUNCTION guard_proxy_auto_allocation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    waiting_message CONSTANT TEXT := '自动代理分配：等待可用节点（每个节点最多 3 个账号）';
    route_changed BOOLEAN;
BEGIN
    IF current_setting('sub2api.proxy_allocator', true) = 'on' THEN
        RETURN NEW;
    END IF;
    IF NEW.deleted_at IS NOT NULL THEN
        NEW.proxy_auto_paused := FALSE;
        RETURN NEW;
    END IF;
    IF NEW.proxy_auto_paused AND (NEW.status <> 'error' OR NEW.schedulable OR NEW.error_message IS DISTINCT FROM waiting_message) THEN
        NEW.proxy_auto_paused := FALSE;
    END IF;
    IF NOT (SELECT enabled FROM proxy_auto_allocation WHERE id = 1) THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'INSERT' THEN
        route_changed := TRUE;
    ELSE
        route_changed := NEW.proxy_id IS DISTINCT FROM OLD.proxy_id
            OR NEW.platform IS DISTINCT FROM OLD.platform OR NEW.type IS DISTINCT FROM OLD.type
            OR NEW.deleted_at IS DISTINCT FROM OLD.deleted_at
            OR NEW.status IS DISTINCT FROM OLD.status OR NEW.schedulable IS DISTINCT FROM OLD.schedulable;
        -- Subscription removal already pauses scheduling. Transfer ownership of
        -- that pause so the allocator can recover it using the remaining pool.
        IF OLD.status = 'active' AND OLD.schedulable AND NEW.proxy_id IS NULL
            AND NEW.error_message = 'Clash 订阅节点已移除，请重新选择代理并恢复账号调度' THEN
            NEW.proxy_auto_paused := TRUE;
            NEW.error_message := waiting_message;
        END IF;
    END IF;
    IF NEW.status = 'active' AND NEW.schedulable AND (route_changed OR NEW.proxy_id IS NULL) THEN
        NEW.proxy_auto_paused := TRUE;
        NEW.status := 'error';
        NEW.schedulable := FALSE;
        NEW.error_message := waiting_message;
    END IF;
    RETURN NEW;
END;
$$;
