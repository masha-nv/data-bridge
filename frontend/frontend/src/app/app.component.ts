import { CommonModule } from '@angular/common';
import { Component, inject } from '@angular/core';
import {
  Router,
  RouterLink,
  RouterLinkActive,
  RouterOutlet,
} from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatMenuModule } from '@angular/material/menu';
import { MatToolbarModule } from '@angular/material/toolbar';

interface AppNavItem {
  label: string;
  path: string;
  enabled: boolean;
  queryParams?: Record<string, string>;
}

interface AppNavGroup {
  label: string;
  path?: string;
  enabled: boolean;
  children?: AppNavItem[];
}

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    CommonModule,
    MatButtonModule,
    MatMenuModule,
    MatToolbarModule,
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss',
})
export class AppComponent {
  private readonly router = inject(Router);

  protected readonly appTitle = 'MARx Maintainer';
  protected readonly navGroups: AppNavGroup[] = [
    {
      label: 'Descriptions',
      path: '/app/descriptions',
      enabled: true,
    },
    {
      label: 'Beneficiaries',
      path: '/app/beneficiaries',
      enabled: true,
    },
    {
      label: 'DevOps',
      enabled: true,
      children: [
        {
          label: 'Active Jobs',
          path: '/app/devops',
          queryParams: { tab: 'active' },
          enabled: true,
        },
        {
          label: 'Completed Jobs',
          path: '/app/devops',
          queryParams: { tab: 'completed' },
          enabled: true,
        },
        {
          label: 'Restart Failed Job',
          path: '/app/devops',
          queryParams: { tab: 'restart-failed-job' },
          enabled: true,
        },
        {
          label: 'Mark Job Complete',
          path: '/app/devops',
          queryParams: { tab: 'mark-job-complete' },
          enabled: true,
        },
        {
          label: 'Submit Batch Job (disabled)',
          path: '/app/submit-batch-job',
          enabled: false,
        },
      ],
    },
    {
      label: 'SQL Runner',
      path: '/app/sql-runner',
      enabled: true,
    },
    {
      label: 'Bene Download',
      path: '/app/bene-download',
      enabled: false,
    },
    {
      label: 'Change Password',
      path: '/app/change-password',
      enabled: false,
    },
    {
      label: 'Tester Utilities',
      path: '/app/tester-utilities',
      enabled: false,
    },
  ];

  protected get isAppRoute(): boolean {
    return this.router.url.startsWith('/app');
  }

  protected isRouteActive(path: string): boolean {
    return this.router.url.startsWith(path);
  }
}
