import copy
import json
from unittest import TestCase
from public_contract_guard import MODELS, LEVELS, validate, parse_pipeline_line, pipeline_matches

class PipelineLogTests(TestCase):
    def test_json_and_existing_console_encoding(self):
        fields = {'generation_request_id': 'request-1', 'account_id': 42, 'timings': {'images': [{'submitted_detail_level': 6}]}}
        expected = dict(fields, msg='adobe_images.pipeline_completed')
        self.assertEqual(parse_pipeline_line(json.dumps(expected)), expected)
        self.assertEqual(parse_pipeline_line('2026-11-07T06:00:00Z\tINFO\thandler/adobe_images.go:800\tadobe_images.pipeline_completed\t' + json.dumps(fields)), expected)

    def test_unrelated_or_malformed_lines_never_correlate(self):
        for line in ['{}', '[]', 'broken', '{"msg":"other"}', 'INFO\tother\t{"message":"adobe_images.pipeline_completed"}', 'INFO\tadobe_images.pipeline_completed\t{broken}']:
            with self.subTest(line=line): self.assertIsNone(parse_pipeline_line(line))

    def test_distinct_generation_http_and_billing_ids_correlate(self):
        entry=dict(msg='adobe_images.pipeline_completed',request_id='http-1',client_request_id='client-1',generation_request_id='generation-1',api_key_id=171,account_id=2206)
        usage=dict(request_id='client:client-1',account_id=2206)
        self.assertTrue(pipeline_matches(entry,usage,'http-1',171))
        for field,value in [('request_id','wrong'),('client_request_id','wrong'),('api_key_id',172),('account_id',99)]:
            with self.subTest(field=field):self.assertFalse(pipeline_matches(dict(entry,**{field:value}),usage,'http-1',171))
        self.assertFalse(pipeline_matches(entry,dict(usage,request_id='generation-1'),'http-1',171))

class PublicQualityContractTests(TestCase):
    def setUp(self):
        self.cases=[]
        for model,version in MODELS.items():
            for quality,level in LEVELS.items():
                self.cases.append(dict(application_sha='a'*40,model=model,quality=quality,success=True,http_status=200,outer_posts=1,response_quality=quality,
                    images=[dict(format='png',width=3840,height=2160,png_fully_validated=True,bytes=2000,sha256='b'*64)],
                    applied_submissions=[dict(submitted_model_version=version,submitted_detail_level=level,submit_attempts=1)],
                    api_key_id=171,http_request_id='http-1',pipeline=dict(request_id='http-1',client_request_id='client-1',api_key_id=171,account_id=2206),
                    pipeline_correlated=True,usage=dict(model=model,image_count=1,request_id='client:client-1',account_id=2206)))

    def test_distinct_verified_levels_pass(self):
        self.assertTrue(validate(self.cases,'a'*40)['all_passed'])

    def test_high_response_or_submission_never_passes(self):
        for change in ['response','level','variant','correlation','attempts','pixels','completeness','duplicate','revision']:
            rows=copy.deepcopy(self.cases)
            if change=='response': rows[0]['response_quality']='high'
            if change=='level': rows[0]['applied_submissions'][0]['submitted_detail_level']=5
            if change=='variant': rows[0]['applied_submissions'][0]['submitted_model_version']='gpt-image-2.6-aurora'
            if change=='correlation': rows[0]['pipeline_correlated']=False
            if change=='attempts': rows[0]['applied_submissions'][0]['submit_attempts']=2
            if change=='pixels': rows[0]['images'][0]['width']=1024
            if change=='completeness': rows[0]['images'][0]['png_fully_validated']=False
            if change=='duplicate': rows[1]=copy.deepcopy(rows[0])
            if change=='revision': rows[0]['application_sha']='c'*40
            with self.subTest(change=change), self.assertRaises(AssertionError):validate(rows,'a'*40)
