import { Routes } from '@angular/router';

import { BeneCopyMovedataComponent } from './features/bene-copy-movedata/bene-copy-movedata.component';
import { BeneficiariesComponent } from './features/beneficiaries/beneficiaries.component';
import { DevopsComponent } from './features/devops/devops.component';
import { DescriptionsComponent } from './features/descriptions/descriptions.component';
import { LoginComponent } from './features/login/login.component';
import { PlaceholderFeatureComponent } from './features/placeholder/placeholder-feature.component';
import { TesterUtilitiesComponent } from './features/tester-utilities/tester-utilities.component';
import { SqlRunnerComponent } from './features/sql-runner/sql-runner.component';

export const routes: Routes = [
	{
		path: '',
		pathMatch: 'full',
		redirectTo: 'login',
	},
	{
		path: 'login',
		title: 'Login',
		component: LoginComponent,
		data: {
			layout: 'auth',
			feature: 'login',
		},
	},
	{
		path: 'app',
		title: 'MARx Maintainer',
		data: {
			layout: 'desktop-shell',
		},
		children: [
			{
				path: '',
				pathMatch: 'full',
				redirectTo: 'descriptions',
			},
			{
				path: 'descriptions',
				title: 'Descriptions',
				component: DescriptionsComponent,
				data: {
					feature: 'descriptions',
					enabled: true,
				},
			},
			{
				path: 'beneficiaries',
				title: 'Beneficiaries',
				component: BeneficiariesComponent,
				data: {
					feature: 'beneficiaries',
					enabled: true,
				},
			},
			{
				path: 'devops',
				title: 'DevOps',
				component: DevopsComponent,
				data: {
					feature: 'devops',
					enabled: true,
				},
			},
			{
				path: 'sql-runner',
				title: 'SQL Runner',
				component: SqlRunnerComponent,
				data: {
					feature: 'sql-runner',
					enabled: true,
				},
			},
			{
				path: 'bene-download-movedata',
				title: 'Bene Copy',
				component: BeneCopyMovedataComponent,
				data: {
					feature: 'bene-download-movedata',
					title: 'Bene Copy',
					enabled: true,
				},
			},
			{
				path: 'change-password',
				title: 'Change Password',
				component: PlaceholderFeatureComponent,
				data: {
					feature: 'change-password',
					title: 'Change Password',
					enabled: false,
					placeholder: true,
				},
			},
			{
				path: 'submit-batch-job',
				title: 'Submit Batch Job',
				component: PlaceholderFeatureComponent,
				data: {
					feature: 'submit-batch-job',
					title: 'Submit Batch Job',
					enabled: false,
					placeholder: true,
				},
			},
			{
				path: 'tester-utilities',
				title: 'Tester Utilities',
				component: TesterUtilitiesComponent,
				data: {
					feature: 'tester-utilities',
					title: 'Tester Utilities',
					enabled: true,
				},
			},
		],
	},
	{
		path: '**',
		redirectTo: 'login',
	},
];
