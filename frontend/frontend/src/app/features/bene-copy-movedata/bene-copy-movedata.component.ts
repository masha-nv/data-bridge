import { CommonModule } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import {
  ChangeDetectionStrategy,
  Component,
  OnDestroy,
  OnInit,
  computed,
  inject,
  signal,
} from '@angular/core';
import {
  AbstractControl,
  FormBuilder,
  ReactiveFormsModule,
  ValidationErrors,
  Validators,
} from '@angular/forms';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { MatSelectModule } from '@angular/material/select';
import { firstValueFrom } from 'rxjs';
import {
  BeneCopyHistoryDetailResponse,
  BeneCopyJobStatus,
  BeneCopyJobStatusResponse,
  BeneCopyJobSubmissionResponse,
  BeneCopyService,
} from '../../services/bene-copy.service';
import { EnvironmentService } from '../../services/environment.service';

@Component({
  selector: 'app-bene-copy-movedata',
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
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './bene-copy-movedata.component.html',
  styleUrl: './bene-copy-movedata.component.scss',
})
export class BeneCopyMovedataComponent implements OnInit, OnDestroy {
  private readonly historyPageSize = 12;
  private readonly formBuilder = inject(FormBuilder);
  private readonly beneCopyService = inject(BeneCopyService);
  private readonly environmentService = inject(EnvironmentService);
  private readonly fallbackEnvironment = 'Dev2';
  private pollTimeoutId: number | null = null;

  readonly environments = this.environmentService.supportedEnvironments;
  readonly selectedSourceEnvironment = signal(this.environmentService.defaultEnvironment());

  readonly submitting = signal(false);
  readonly polling = signal(false);
  readonly errorMessage = signal('');
  readonly job = signal<BeneCopyJobStatusResponse | null>(null);
  readonly historyLoading = signal(false);
  readonly historyErrorMessage = signal('');
  readonly recentJobs = signal<BeneCopyJobStatusResponse[]>([]);
  readonly selectedHistoryJobId = signal<string | null>(null);
  readonly selectedHistoryDetail = signal<BeneCopyHistoryDetailResponse | null>(null);

  readonly form = this.formBuilder.nonNullable.group({
    sourceEnvironment: [this.environmentService.defaultEnvironment(), Validators.required],
    targetEnvironment: [this.fallbackEnvironment, Validators.required],
    beneLinkPartKey: ['', [Validators.required, Validators.pattern(/^\d+$/)]],
    beneLinkKey: ['', [Validators.required, Validators.pattern(/^\d+$/)]],
  }, {
    validators: [BeneCopyMovedataComponent.environmentsMustDifferValidator],
  });

  readonly targetEnvironments = computed(() => {
    const sourceEnvironment = this.selectedSourceEnvironment();
    return this.getAvailableTargetEnvironments(sourceEnvironment);
  });

  readonly activeJob = computed(() => {
    const currentJob = this.job();
    if (!currentJob || this.isTerminalStatus(currentJob.status)) {
      return null;
    }
    return currentJob;
  });

  readonly terminalJob = computed(() => {
    const currentJob = this.job();
    if (!currentJob || !this.isTerminalStatus(currentJob.status)) {
      return null;
    }
    return currentJob;
  });

  readonly hasSubmittedJob = computed(() => this.job() !== null);
  readonly selectedHistoryJob = computed(() => this.selectedHistoryDetail()?.job ?? null);
  readonly selectedHistoryTables = computed(() => this.selectedHistoryDetail()?.tables ?? []);
  readonly hasHistory = computed(() => this.recentJobs().length > 0);

  async ngOnInit(): Promise<void> {
    await this.environmentService.ensureLoaded();

    if (this.environmentService.errorMessage()) {
      this.errorMessage.set(this.environmentService.errorMessage());
      return;
    }

    const sourceEnvironment = this.form.controls.sourceEnvironment.value;
    if (!this.environments().includes(sourceEnvironment)) {
      this.form.patchValue({
        sourceEnvironment: this.environmentService.defaultEnvironment(),
      });
    }

    this.selectedSourceEnvironment.set(this.form.controls.sourceEnvironment.value);
    this.ensureValidTargetEnvironment();
    await this.loadHistory();
  }

  ngOnDestroy(): void {
    this.stopPolling();
  }

  get isSubmitDisabled(): boolean {
    return this.form.invalid || this.submitting() || this.polling();
  }

  get sameEnvironmentSelected(): boolean {
    return !!this.form.errors?.['sameEnvironment'];
  }

  async copy(): Promise<void> {
    this.form.markAllAsTouched();
    if (this.form.invalid) {
      this.errorMessage.set('Choose different source and target environments and enter numeric Bene Link Partition Key and Bene Link Key values.');
      return;
    }

    this.stopPolling();
    this.submitting.set(true);
    this.errorMessage.set('');
    this.job.set(null);

    try {
      const response = await firstValueFrom(this.beneCopyService.submitMovedataJob(this.form.getRawValue()));
      this.job.set(this.buildInitialJobStatus(response));
      this.startPolling(response.jobId);
    } catch (error: unknown) {
      this.errorMessage.set(this.getCopyErrorMessage(error));
    } finally {
      this.submitting.set(false);
    }
  }

  reset(): void {
    this.stopPolling();
    this.form.reset({
      sourceEnvironment: this.environmentService.defaultEnvironment(),
      targetEnvironment: this.getDefaultTargetEnvironment(),
      beneLinkPartKey: '',
      beneLinkKey: '',
    });
    this.form.markAsPristine();
    this.form.markAsUntouched();
    this.errorMessage.set('');
    this.job.set(null);
  }

  async selectHistoryJob(jobId: string): Promise<void> {
    await this.loadHistoryDetail(jobId);
  }

  formatDuration(durationMs?: number): string {
    if (!durationMs || durationMs <= 0) {
      return 'n/a';
    }

    const totalSeconds = Math.floor(durationMs / 1000);
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;

    if (hours > 0) {
      return `${hours}h ${minutes}m ${seconds}s`;
    }

    if (minutes > 0) {
      return `${minutes}m ${seconds}s`;
    }

    return `${seconds}s`;
  }

  syncTargetEnvironment(): void {
    this.selectedSourceEnvironment.set(this.form.controls.sourceEnvironment.value);
    this.ensureValidTargetEnvironment();
  }

  private ensureValidTargetEnvironment(): void {
    const selectedTarget = this.form.controls.targetEnvironment.value;
    if (!this.targetEnvironments().includes(selectedTarget)) {
      this.form.patchValue({
        targetEnvironment: this.getDefaultTargetEnvironment(),
      });
    }
  }

  private getDefaultTargetEnvironment(): string {
    return this.getAvailableTargetEnvironments(this.form.controls.sourceEnvironment.value)[0] ?? this.fallbackEnvironment;
  }

  private getAvailableTargetEnvironments(sourceEnvironment: string): string[] {
    return this.environments().filter(
      (environment) => environment !== 'Prod2' && environment !== sourceEnvironment,
    );
  }

  private static environmentsMustDifferValidator(control: AbstractControl): ValidationErrors | null {
    const sourceEnvironment = control.get('sourceEnvironment')?.value;
    const targetEnvironment = control.get('targetEnvironment')?.value;

    if (!sourceEnvironment || !targetEnvironment) {
      return null;
    }

    return sourceEnvironment === targetEnvironment ? { sameEnvironment: true } : null;
  }

  private getCopyErrorMessage(error: unknown): string {
    if (error instanceof HttpErrorResponse) {
      if (typeof error.error === 'string' && error.error.trim() !== '') {
        return error.error;
      }

      if (error.status === 0) {
        return 'Backend is unavailable. Start the Electron demo backend and try again.';
      }
    }

    return 'Unable to run movedata bene copy right now. Try again.';
  }

  private buildInitialJobStatus(response: BeneCopyJobSubmissionResponse): BeneCopyJobStatusResponse {
    return {
      jobId: response.jobId,
      engine: response.engine,
      status: response.status,
      submittedAt: response.submittedAt,
      updatedAt: response.submittedAt,
      sourceEnvironment: response.sourceEnvironment,
      targetEnvironment: response.targetEnvironment,
      beneLinkPartKey: response.beneLinkPartKey,
      beneLinkKey: response.beneLinkKey,
      copiedRows: 0,
      skippedRows: 0,
      message: response.message,
    };
  }

  private startPolling(jobId: string): void {
    this.polling.set(true);
    void this.pollJobStatus(jobId);
  }

  private async pollJobStatus(jobId: string): Promise<void> {
    try {
      const status = await firstValueFrom(this.beneCopyService.getJobStatus(jobId));
      this.job.set(status);

      if (this.isTerminalStatus(status.status)) {
        this.polling.set(false);
        this.pollTimeoutId = null;
        await this.loadHistory(status.jobId);
        return;
      }

      this.pollTimeoutId = window.setTimeout(() => {
        void this.pollJobStatus(jobId);
      }, 2000);
    } catch (error: unknown) {
      this.polling.set(false);
      this.pollTimeoutId = null;
      this.errorMessage.set(this.getCopyErrorMessage(error));
    }
  }

  private stopPolling(): void {
    this.polling.set(false);
    if (this.pollTimeoutId !== null) {
      window.clearTimeout(this.pollTimeoutId);
      this.pollTimeoutId = null;
    }
  }

  private isTerminalStatus(status: BeneCopyJobStatus): boolean {
    return status === 'completed' || status === 'failed';
  }

  private async loadHistory(preferredJobId?: string): Promise<void> {
    this.historyLoading.set(true);
    this.historyErrorMessage.set('');

    try {
      const response = await firstValueFrom(this.beneCopyService.getHistory(this.historyPageSize, 'movedata'));
      this.recentJobs.set(response.jobs);

      const jobIdToLoad = preferredJobId ?? this.selectedHistoryJobId() ?? response.jobs[0]?.jobId ?? null;
      if (jobIdToLoad) {
        await this.loadHistoryDetail(jobIdToLoad);
      } else {
        this.selectedHistoryJobId.set(null);
        this.selectedHistoryDetail.set(null);
      }
    } catch (error: unknown) {
      this.historyErrorMessage.set(this.getCopyErrorMessage(error));
    } finally {
      this.historyLoading.set(false);
    }
  }

  private async loadHistoryDetail(jobId: string): Promise<void> {
    this.selectedHistoryJobId.set(jobId);

    try {
      const detail = await firstValueFrom(this.beneCopyService.getJobHistory(jobId));
      this.selectedHistoryDetail.set(detail);
      this.historyErrorMessage.set('');
    } catch (error: unknown) {
      this.selectedHistoryDetail.set(null);
      this.historyErrorMessage.set(this.getCopyErrorMessage(error));
    }
  }
}