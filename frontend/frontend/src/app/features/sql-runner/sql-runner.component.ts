import { CommonModule } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import {
  ChangeDetectionStrategy,
  Component,
  OnInit,
  inject,
  signal,
} from '@angular/core';
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
import { firstValueFrom } from 'rxjs';
import {
  SqlRunnerResponse,
  SqlRunnerService,
  SqlStatementResult,
} from '../../services/sql-runner.service';
import { EnvironmentService } from '../../services/environment.service';

@Component({
  selector: 'app-sql-runner',
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
  templateUrl: './sql-runner.component.html',
  styleUrls: ['./sql-runner.component.scss'],
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SqlRunnerComponent implements OnInit {
  private readonly formBuilder = inject(FormBuilder);
  private readonly environmentService = inject(EnvironmentService);
  private readonly sqlRunnerService = inject(SqlRunnerService);

  readonly environments = this.environmentService.supportedEnvironments;
  readonly loading = signal(false);
  readonly errorMessage = signal('');
  readonly result = signal<SqlRunnerResponse | null>(null);

  readonly form = this.formBuilder.nonNullable.group({
    environment: [this.environmentService.defaultEnvironment(), Validators.required],
    sql: [
      'select * from mcs_tran_reply;\nselect * from cme_bene_stus;',
      Validators.required,
    ],
  });

  async ngOnInit(): Promise<void> {
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
  }

  get isSubmitDisabled(): boolean {
    return this.form.invalid || this.loading();
  }

  async runSql(): Promise<void> {
    this.form.markAllAsTouched();
    if (this.form.invalid) {
      this.errorMessage.set('Choose an environment and enter one or more SQL statements.');
      return;
    }

    this.loading.set(true);
    this.errorMessage.set('');
    this.result.set(null);

    try {
      const response = await firstValueFrom(
        this.sqlRunnerService.run(
          this.form.getRawValue().environment,
          this.form.getRawValue().sql,
        ),
      );

      this.result.set(response);
    } catch (error: unknown) {
      this.errorMessage.set(this.getRunErrorMessage(error));
    } finally {
      this.loading.set(false);
    }
  }

  trackStatement(_: number, result: SqlStatementResult): number {
    return result.statementNumber;
  }

  private getRunErrorMessage(error: unknown): string {
    if (error instanceof HttpErrorResponse) {
      if (typeof error.error === 'string' && error.error.trim() !== '') {
        return error.error;
      }

      if (error.status === 0) {
        return 'Backend is unavailable. Start the Electron demo backend and try again.';
      }
    }

    return 'Unable to run SQL right now. Try again.';
  }
}