#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { DnsStack } from '../lib/dns-stack';

const app = new cdk.App();

new DnsStack(app, 'MoneyBaeDns', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    // Must be us-east-1: ACM certificates used by CloudFront have to be created here.
    region: 'us-east-1',
  },
});
