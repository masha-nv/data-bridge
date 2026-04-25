import { CommonModule } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  inject,
  signal,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import {
  FormBuilder,
  ReactiveFormsModule,
  Validators,
} from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSelectModule } from '@angular/material/select';
import { MatTabsModule } from '@angular/material/tabs';
import { ActivatedRoute, Router } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import {
  DevopsActionResponse,
  DevopsJobRecord,
  DevopsService,
} from '../../services/devops.service';
import { EnvironmentService } from '../../services/environment.service';

@Component({
  selector: 'app-devops',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatInputModule,
    MatProgressSpinnerModule,
    MatSelectModule,
    MatTabsModule,
  ],
  templateUrl: './devops.component.html',
  styleUrl: './devops.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class DevopsComponent implements OnInit {
  private readonly route = inject(ActivatedRoute);
  private readonly router = inject(Router);
  private readonly formBuilder = inject(FormBuilder);
  private readonly environmentService = inject(EnvironmentService);
  private readonly devopsService = inject(DevopsService);

  readonly environments = this.environmentService.supportedEnvironments;
  readonly loading = signal(false);
  readonly actionSubmitting = signal(false);
  readonly errorMessage = signal('');
  readonly actionErrorMessage = signal('');
  readonly actionResult = signal<DevopsActionResponse | null>(null);
  readonly selectedTabIndex = signal(0);
  readonly activeRows = signal<DevopsJobRecord[]>([]);
  readonly completedRows = signal<DevopsJobRecord[]>([]);
  readonly activeColumns = [
    'statusCode',
    'jobId',
    'batchName',
    'threadKey',
    'inFilePath',
    'statusDateTime',
  ];
  readonly completedColumns = [
    'statusCode',
    'jobId',
    'batchName',
    'threadKey',
    'endDateTime',
    'statusDateTime',
  ];

  readonly form = this.formBuilder.nonNullable.group({
    environment: [this.environmentService.defaultEnvironment(), Validators.required],
  });
  readonly restartForm = this.formBuilder.nonNullable.group({
    jobIds: ['', Validators.required],
  });
  readonly markCompleteForm = this.formBuilder.nonNullable.group({
    currentStatus: ['2', Validators.required],
    jobIds: ['', Validators.required],
  });

  async ngOnInit(): Promise<void> {
    this.route.queryParamMap
      .pipe(takeUntilDestroyed())
      .subscribe((queryParams) => {
        this.selectedTabIndex.set(
          this.getTabIndex(queryParams.get('tab') ?? 'active'),
        );
      });

    await this.environmentService.ensureLoaded();

    if (this.environmentService.errorMessage()) {
      this.errorMessage.set(this.environmentService.errorMessage());
      return;
    }

    const availableEnvironments = this.environments();
    const selectedEnvironment = this.form.controls.environment.value;
    if (!availableEnvironments.includes(selectedEnvironment)) {
      this.form.patchValue({
        environment: this.environmentService.defaultEnvironment(),
      });
    }

    await this.loadJobs();
  }

  async loadJobs(): Promise<void> {
    this.form.markAllAsTouched();
    if (this.form.invalid) {
      this.errorMessage.set('Choose an environment to load DevOps jobs.');
      return;
    }

    const environment = this.form.getRawValue().environment;
    this.loading.set(true);
    this.errorMessage.set('');

    try {
      const [activeResponse, completedResponse] = await Promise.all([
        firstValueFrom(this.devopsService.getJobs(environment, 'active')),
        firstValueFrom(this.devopsService.getJobs(environment, 'completed')),
      ]);

      this.activeRows.set(activeResponse.rows);
      this.completedRows.set(completedResponse.rows);
    } catch (error: unknown) {
      this.errorMessage.set(this.getLoadErrorMessage(error));
      this.activeRows.set([]);
      this.completedRows.set([]);
    } finally {
      this.loading.set(false);
    }
  }

  async restartFailedJobs(): Promise<void> {
    this.restartForm.markAllAsTouched();
    if (this.restartForm.invalid || this.form.invalid) {
      this.actionErrorMessage.set('Enter one or more job IDs to restart.');
      return;
    }

    await this.runAction(() =>
      firstValueFrom(
        this.devopsService.restartJobs(
          this.form.getRawValue().environment,
          this.restartForm.getRawValue().jobIds,
        ),
      ),
    );

    if (!this.actionErrorMessage()) {
      this.restartForm.patchValue({ jobIds: '' });
      await this.loadJobs();
    }
  }

  async markSelectedJobsComplete(): Promise<void> {
    this.markCompleteForm.markAllAsTouched();
    if (this.markCompleteForm.invalid || this.form.invalid) {
      this.actionErrorMessage.set(
        'Enter a current status code and one or more job IDs to mark complete.',
      );
      return;
    }

    await this.runAction(() =>
      firstValueFrom(
        this.devopsService.markJobsComplete(
          this.form.getRawValue().environment,
          this.markCompleteForm.getRawValue().currentStatus,
          this.markCompleteForm.getRawValue().jobIds,
        ),
      ),
    );

    if (!this.actionErrorMessage()) {
      this.markCompleteForm.patchValue({
        currentStatus: '2',
        jobIds: '',
      });
      await this.loadJobs();
    }
  }

  trackJob(_: number, row: DevopsJobRecord): number {
    return row.threadId;
  }

  handleTabChange(index: number): void {
    const tab = this.getTabName(index);
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { tab },
      queryParamsHandling: 'merge',
    });
  }

  private getLoadErrorMessage(error: unknown): string {
    if (error instanceof HttpErrorResponse) {
      if (typeof error.error === 'string' && error.error.trim() !== '') {
        return error.error;
      }

      if (error.status === 0) {
        return 'Backend is unavailable. Start the Electron demo backend and try again.';
      }
    }

    return 'Unable to load DevOps jobs right now. Try again.';
  }

  private async runAction(
    action: () => Promise<DevopsActionResponse>,
  ): Promise<void> {
    this.actionSubmitting.set(true);
    this.actionErrorMessage.set('');
    this.actionResult.set(null);

    try {
      const response = await action();
      this.actionResult.set(response);
    } catch (error: unknown) {
      this.actionErrorMessage.set(this.getActionErrorMessage(error));
    } finally {
      this.actionSubmitting.set(false);
    }
  }

  private getActionErrorMessage(error: unknown): string {
    if (error instanceof HttpErrorResponse) {
      if (typeof error.error === 'string' && error.error.trim() !== '') {
        return error.error;
      }

      if (error.status === 0) {
        return 'Backend is unavailable. Start the Electron demo backend and try again.';
      }
    }

    return 'Unable to run the DevOps action right now. Try again.';
  }

  private getTabIndex(tab: string): number {
    switch (tab) {
      case 'completed':
        return 1;
      case 'restart-failed-job':
        return 2;
      case 'mark-job-complete':
        return 3;
      default:
        return 0;
    }
  }

  private getTabName(index: number): string {
    switch (index) {
      case 1:
        return 'completed';
      case 2:
        return 'restart-failed-job';
      case 3:
        return 'mark-job-complete';
      default:
        return 'active';
    }
  }
}