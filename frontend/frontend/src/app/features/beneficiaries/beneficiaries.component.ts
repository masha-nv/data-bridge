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

interface BeneficiaryLookupResponse {
  environment: string;
  idType: string;
  idValue: string;
  beneLinkPartKey: number;
  beneLinkKey: number;
  hicn: string;
  beneDeathDate: string;
  beneBirthDate: string;
  beneLastName: string;
  beneFirstName: string;
  middleName: string;
  ssn: string;
  beneSex: string;
  archiveStatus: string;
  lastUpdateTs: string;
  mbi: string;
  rrbHicn: string;
}

@Component({
  selector: 'app-beneficiaries',
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
  templateUrl: './beneficiaries.component.html',
  styleUrl: './beneficiaries.component.scss',
})
export class BeneficiariesComponent implements OnInit {
  private readonly formBuilder = inject(FormBuilder);
  private readonly httpClient = inject(HttpClient);
  private readonly environmentService = inject(EnvironmentService);

  readonly environments = this.environmentService.supportedEnvironments;
  readonly idTypes = ['BLK', 'MBI', 'HICN', 'SSN', 'RRB-HICN'];

  readonly loading = signal(false);
  readonly errorMessage = signal('');
  readonly result = signal<BeneficiaryLookupResponse | null>(null);

  readonly form = this.formBuilder.nonNullable.group({
    environment: [this.environmentService.defaultEnvironment(), Validators.required],
    idType: ['BLK', Validators.required],
    idValue: ['', Validators.required],
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

  async lookup(): Promise<void> {
    this.form.markAllAsTouched();
    if (this.form.invalid) {
      this.errorMessage.set('Choose an environment, ID type, and beneficiary ID to continue.');
      return;
    }

    this.loading.set(true);
    this.errorMessage.set('');
    this.result.set(null);

    try {
      const response = await firstValueFrom(
        this.httpClient.post<BeneficiaryLookupResponse>(
          this.environmentService.getApiUrl('/api/marx/beneficiaries/lookup'),
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
      idType: 'BLK',
      idValue: '',
    });
    this.form.markAsPristine();
    this.form.markAsUntouched();
    this.errorMessage.set('');
    this.result.set(null);
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

    return 'Unable to load beneficiary details right now. Try again.';
  }
}