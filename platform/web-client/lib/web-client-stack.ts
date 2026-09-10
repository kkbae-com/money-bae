import * as path from 'path';
import * as cdk from 'aws-cdk-lib';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as s3deploy from 'aws-cdk-lib/aws-s3-deployment';
import * as acm from 'aws-cdk-lib/aws-certificatemanager';
import * as route53 from 'aws-cdk-lib/aws-route53';
import * as targets from 'aws-cdk-lib/aws-route53-targets';
import { Construct } from 'constructs';

// Hardcoded to the dev domain — this stack is deployed once, as
// MoneyBaeWebClient-Dev. A future -c env=prod deploy would collide on
// this CloudFront alias (CNAMEAlreadyExists); parameterize this per
// env (e.g. a stack prop) before a second environment is ever deployed.
const SITE_DOMAIN = 'money.kkbae.com';
// From platform/dns's `MoneyBaeDns` stack's `HostedZoneId` output (Task 2
// of docs/superpowers/plans/2026-09-07-platform-dns-implementation.md).
const HOSTED_ZONE_ID = 'Z03259513H7JI1AA99B6V';

export class WebClientStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    const zone = route53.HostedZone.fromHostedZoneAttributes(this, 'Zone', {
      hostedZoneId: HOSTED_ZONE_ID,
      zoneName: 'kkbae.com',
    });

    const certificate = new acm.Certificate(this, 'SiteCertificate', {
      domainName: SITE_DOMAIN,
      validation: acm.CertificateValidation.fromDns(zone),
    });

    const bucket = new s3.Bucket(this, 'SiteBucket', {
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
    });

    const distribution = new cloudfront.Distribution(this, 'SiteDistribution', {
      domainNames: [SITE_DOMAIN],
      certificate,
      defaultRootObject: 'index.html',
      defaultBehavior: {
        origin: origins.S3BucketOrigin.withOriginAccessControl(bucket),
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
      },
      // TanStack Router does client-side routing, so any path CloudFront can't
      // find in the bucket (a deep link like /ledger, or S3 rejecting a
      // private-bucket miss as 403) needs to fall back to index.html and let
      // the SPA's router take over.
      errorResponses: [
        { httpStatus: 403, responseHttpStatus: 200, responsePagePath: '/index.html' },
        { httpStatus: 404, responseHttpStatus: 200, responsePagePath: '/index.html' },
      ],
    });

    new route53.ARecord(this, 'SiteAliasRecordV4', {
      zone,
      recordName: SITE_DOMAIN,
      target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
    });
    new route53.AaaaRecord(this, 'SiteAliasRecordV6', {
      zone,
      recordName: SITE_DOMAIN,
      target: route53.RecordTarget.fromAlias(new targets.CloudFrontTarget(distribution)),
    });

    new s3deploy.BucketDeployment(this, 'SiteDeployment', {
      sources: [s3deploy.Source.asset(path.join(__dirname, '../../../clients/app/dist'))],
      destinationBucket: bucket,
      distribution,
      distributionPaths: ['/*'],
    });

    new cdk.CfnOutput(this, 'DistributionDomainName', {
      value: distribution.distributionDomainName,
    });
    new cdk.CfnOutput(this, 'SiteUrl', { value: `https://${SITE_DOMAIN}` });
  }
}
