import copy
import importlib.util
from pathlib import Path
from unittest import TestCase
spec=importlib.util.spec_from_file_location('review',Path(__file__).with_name('review_concurrent_okad.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class ConcurrentOkadReviewTests(TestCase):
    def setUp(self):
        self.before={'at':'original','containers':{m.APP:{'id':'app','image_id':'app-image'},m.OKAD:{'id':'old','image_id':'old-image','resources':{'Memory':3*1024**3},'go_env':{}}},'config_hashes':{m.COMPOSE:'compose-0','nginx':'unchanged'},'settings_hashes':{'smtp':'unchanged'}}
        self.receipts=[]
        for n,name in enumerate(m.RELEASES):
            old='old' if n==0 else 'okad-1'
            prior={'container':old,'source_image_id':old+'-image','config_hashes':{m.COMPOSE:'compose-'+str(n),'env':'same'},'database':{'settings':'same'},'source_hashes':{'a.py':str(n)}}
            after={'container':'okad-'+str(n+1),'image_id':'okad-'+str(n+1)+'-image','image':'adobe-team-cookie:'+name,'config_hashes':{m.COMPOSE:'compose-'+str(n+1),'env':'same'},'database':{'settings':'same'},'source_hashes':{'a.py':str(n+1)},'changed_setting_keys':[],'verification':{'api':'ok','database':'ok'},'other_containers':{'/'+m.APP:{'id':'app','image':'app-image'}}}
            self.receipts.append({'manifest':{'release':name,'container':old,'source_image_id':old+'-image','files':{'a.py':{'before':str(n),'after':str(n+1)}}},'before':prior,'after':after})
        self.current=copy.deepcopy(self.before)
        self.current['containers'][m.OKAD].update({'id':'okad-2','image_id':'okad-2-image','image':'adobe-team-cookie:'+m.RELEASES[-1],'health':'healthy','restarts':0})
        self.current['config_hashes'][m.COMPOSE]='compose-2'

    def test_only_reviewed_identity_and_compose_are_updated(self):
        original=copy.deepcopy(self.before)
        adjusted=m.reviewed_baseline(self.before,self.current,self.receipts)
        self.assertEqual(self.before,original)
        self.assertEqual(adjusted,self.current)

    def test_unreviewed_later_container_is_rejected(self):
        self.current['containers'][m.OKAD]['id']='later'
        with self.assertRaises(AssertionError):m.reviewed_baseline(self.before,self.current,self.receipts)

    def test_resource_change_is_rejected(self):
        self.current['containers'][m.OKAD]['resources']['Memory']=1
        with self.assertRaises(AssertionError):m.reviewed_baseline(self.before,self.current,self.receipts)

    def test_setting_change_is_rejected(self):
        self.receipts[-1]['after']['database']['settings']='changed'
        with self.assertRaises(AssertionError):m.reviewed_baseline(self.before,self.current,self.receipts)

    def test_unlisted_source_change_is_rejected(self):
        self.receipts[-1]['after']['source_hashes']['unlisted.py']='new'
        with self.assertRaises(AssertionError):m.reviewed_baseline(self.before,self.current,self.receipts)

    def test_broken_release_chain_is_rejected(self):
        self.receipts[-1]['manifest']['source_image_id']='unrelated'
        with self.assertRaises(AssertionError):m.reviewed_baseline(self.before,self.current,self.receipts)

    def test_other_application_replacement_is_rejected(self):
        self.receipts[-1]['after']['other_containers']['/'+m.APP]['id']='other'
        with self.assertRaises(AssertionError):m.reviewed_baseline(self.before,self.current,self.receipts)
