import { CommonModule } from '@angular/common';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
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
import { EnvironmentService } from '../../services/environment.service';

interface DescriptionLookupResponse {
  environment: string;
  type: string;
  code: string;
  description: string;
}

@Component({
  selector: 'app-descriptions',
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
  templateUrl: './descriptions.component.html',
  styleUrl: './descriptions.component.scss',
})
export class DescriptionsComponent implements OnInit {
  private readonly formBuilder = inject(FormBuilder);
  private readonly httpClient = inject(HttpClient);
  private readonly environmentService = inject(EnvironmentService);

  readonly environments = this.environmentService.supportedEnvironments;
  readonly descriptionTypes = ['TRC', 'PW-R Reply Codes'];

  readonly loading = signal(false);
  readonly errorMessage = signal('');
  readonly result = signal<DescriptionLookupResponse | null>(null);

  readonly form = this.formBuilder.nonNullable.group({
    environment: [this.environmentService.defaultEnvironment(), Validators.required],
    type: ['TRC', Validators.required],
    code: ['', Validators.required],
  });

  async ngOnInit(): Promise<void> {
    await this.environmentService.ensureLoaded();

    if (this.environmentService.errorMessage()) {
      this.errorMessage.set(this.environmentService.errorMessage());
      return;
    }

    const selectedEnvironment = this.form.controls.environment.value;
    const availableEnvironments = this.environments();

    if (!availableEnvironments.includes(selectedEnvironment)) {
      this.form.patchValue({
        environment: this.environmentService.defaultEnvironment(),
      });
    }
  }

  get isSubmitDisabled(): boolean {
    return this.form.invalid || this.loading();
  }

  async lookup(): Promise<void> {
    this.form.markAllAsTouched();
    if (this.form.invalid) {
      this.errorMessage.set('Choose an environment, type, and code to continue.');
      return;
    }

    this.loading.set(true);
    this.errorMessage.set('');
    this.result.set(null);

    try {
      const response = await firstValueFrom(
        this.httpClient.post<DescriptionLookupResponse>(
          this.getLookupEndpoint(),
          this.form.getRawValue(),
        ),
      );

      this.result.set(response);
    } catch (error: unknown) {
      this.errorMessage.set(this.getLookupErrorMessage(error));
    } finally {
      this.loading.set(false);
    }
  }

  reset(): void {
    this.form.reset({
      environment: this.environmentService.defaultEnvironment(),
      type: 'TRC',
      code: '',
    });
    this.form.markAsPristine();
    this.form.markAsUntouched();
    this.errorMessage.set('');
    this.result.set(null);
  }

  private getLookupEndpoint(): string {
    return this.environmentService.getApiUrl('/api/marx/descriptions/lookup');
  }

  private getLookupErrorMessage(error: unknown): string {
    if (error instanceof HttpErrorResponse) {
      if (typeof error.error === 'string' && error.error.trim() !== '') {
        return error.error;
      }

      if (error.status === 0) {
        return 'Backend is unavailable. Start the Electron demo backend and try again.';
      }
    }

    return 'Unable to load the description right now. Try again.';
  }
}