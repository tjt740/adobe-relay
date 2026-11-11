import importlib.util
from pathlib import Path
from unittest import TestCase, mock

spec=importlib.util.spec_from_file_location('rolling',Path(__file__).with_name('rolling.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class RollingHandoffTests(TestCase):
    def test_database_connection_pressure_is_unknown_not_zero(self):
        with mock.patch.object(m,'run',side_effect=RuntimeError('FATAL: sorry, too many clients already')):
            self.assertIsNone(m.running_checks_observation())
        with mock.patch.object(m,'run',side_effect=RuntimeError('authentication failed')):
            with self.assertRaises(RuntimeError):m.running_checks_observation()

    def test_unknown_database_observation_never_completes_drain(self):
        with mock.patch.object(m,'own_lease'), mock.patch.object(m,'load',return_value={'old_id':'old','old_prefixes':['prefix'],'canary_prefix':'new'}), mock.patch.object(m,'inspect',return_value={'Id':'old'}), mock.patch.object(m,'slot_prefixes',return_value={}), mock.patch.object(m,'pending_uploads',return_value=0), mock.patch.object(m,'running_checks_observation',return_value=None), mock.patch.object(m,'available',return_value=10*1024**3), mock.patch.object(m.time,'monotonic',side_effect=[0,1,1501]), mock.patch.object(m,'save') as save:
            with self.assertRaises(AssertionError):m.drain('old')
            self.assertEqual([c.args[0] for c in save.call_args_list],['rolling-old-drain-progress.json'])
            self.assertIsNone(save.call_args.args[1]['samples'][0]['running_automatic_checks'])

    def test_default_root_user_null_and_empty_are_same_startup_contract(self):
        self.assertEqual(m.startup_contract({'User':None}),m.startup_contract({'User':''}))
        self.assertNotEqual(m.startup_contract({'User':'1000'}),m.startup_contract({'User':''}))

    def test_canary_preserves_security_and_log_rotation(self):
        opts=m.preserved_container_options({'HostConfig':{'SecurityOpt':['no-new-privileges:true'],'LogConfig':{'Type':'json-file','Config':{'max-file':'3','max-size':'10m'}}}})
        self.assertEqual(opts,['--security-opt','no-new-privileges:true','--log-driver','json-file','--log-opt','max-file=3','--log-opt','max-size=10m'])

    def test_unhealthy_canary_does_not_change_routing(self):
        with mock.patch.object(m,'locks'), mock.patch.object(m,'own_lease'), mock.patch.object(m,'load',return_value={'old_id':'old'}), mock.patch.object(m,'inspect',return_value={'Id':'old'}), mock.patch.object(m,'health',return_value=False), mock.patch.object(m,'run') as run:
            with self.assertRaises(AssertionError):m.switch(True)
            run.assert_not_called()

    def test_failed_canonical_health_restores_canary_redirect(self):
        with mock.patch.object(m,'locks'), mock.patch.object(m,'own_lease'), mock.patch.object(m.Path,'read_text',return_value='expected'), mock.patch.object(m,'inspect',return_value={'Config':{'Image':'adobe-relay:expected'},'State':{'Health':{'Status':'healthy'}}}), mock.patch.object(m,'rule_present',side_effect=[True,False]), mock.patch.object(m,'health',return_value=False), mock.patch.object(m,'run') as run:
            with self.assertRaises(AssertionError):m.switch(False)
            self.assertEqual(run.call_args_list[0].args[0][3:5],['-D','OUTPUT'])
            self.assertEqual(run.call_args_list[1].args[0][3:6],['-I','OUTPUT','1'])

    def test_foreign_canary_is_not_stopped(self):
        with mock.patch.object(m,'locks'), mock.patch.object(m,'rule_present',return_value=False), mock.patch.object(m.Path,'exists',return_value=True), mock.patch.object(m,'load',return_value={'canary_id':'original'}), mock.patch.object(m,'inspect',return_value={'Id':'later'}), mock.patch.object(m,'run') as run:
            with self.assertRaises(AssertionError):m.cleanup()
            run.assert_not_called()

    def test_unknown_source_owner_is_not_declared_drained(self):
        with mock.patch.object(m,'own_lease'), mock.patch.object(m,'load',return_value={'old_id':'original','old_prefixes':['prefix']}), mock.patch.object(m,'inspect',return_value={'Id':'later'}), mock.patch.object(m,'save') as save:
            with self.assertRaises(AssertionError):m.drain('old')
            save.assert_not_called()
