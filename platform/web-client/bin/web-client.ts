#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { WebClientStack } from '../lib/web-client-stack';

const app = new cdk.App();

const env: string = app.node.tryGetContext('env') ?? 'dev';
const stackSuffix = env.charAt(0).toUpperCase() + env.slice(1);

new WebClientStack(app, `MoneyBaeWebClient-${stackSuffix}`, {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: 'us-east-1',
  },
});
