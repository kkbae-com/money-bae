import * as cdk from 'aws-cdk-lib';
import * as route53 from 'aws-cdk-lib/aws-route53';
import { Construct } from 'constructs';

export class DnsStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const zone = new route53.HostedZone(this, 'KkbaeZone', {
      zoneName: 'kkbae.com',
    });
    zone.applyRemovalPolicy(cdk.RemovalPolicy.RETAIN);

    new cdk.CfnOutput(this, 'HostedZoneId', { value: zone.hostedZoneId });
    new cdk.CfnOutput(this, 'NameServers', {
      value: cdk.Fn.join(', ', zone.hostedZoneNameServers!),
    });
  }
}
