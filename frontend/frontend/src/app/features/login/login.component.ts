import { CommonModule } from '@angular/common';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import {
  ChangeDetectionStrategy,
  Component,
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
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';
import { Router } from '@angular/router';
import { AuthService } from '../../services/auth.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressSpinnerModule,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './login.component.html',
  styleUrl: './login.component.scss',
})
export class LoginComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly router = inject(Router);
  private readonly authService = inject(AuthService);

  readonly submitting = signal(false);
  readonly errorMessage = signal('');

  readonly form = this.formBuilder.nonNullable.group({
    userId: ['', Validators.required],
    password: ['', Validators.required],
  });

  get isSubmitDisabled(): boolean {
    return this.form.invalid || this.submitting();
  }

  get userIdControl() {
    return this.form.controls.userId;
  }

  get passwordControl() {
    return this.form.controls.password;
  }

  async handleSubmit(): Promise<void> {
    this.form.markAllAsTouched();
    if (this.form.invalid) {
      this.errorMessage.set('Enter a user ID and password to continue.');
      return;
    }

    this.errorMessage.set('');
    this.submitting.set(true);

    try {
      const response = await this.authService.login(this.form.getRawValue());

      if (!response.connected) {
        this.errorMessage.set('Login did not establish a session.');
        return;
      }

      await this.router.navigateByUrl('/app/descriptions');
    } catch (error: unknown) {
      this.errorMessage.set(this.getLoginErrorMessage(error));
    } finally {
      this.submitting.set(false);
    }
  }

  resetForm(): void {
    this.form.reset({
      userId: '',
      password: '',
    });
    this.form.markAsPristine();
    this.form.markAsUntouched();
    this.errorMessage.set('');
  }

  private getLoginErrorMessage(error: unknown): string {
    if (error instanceof HttpErrorResponse) {
      if (typeof error.error === 'string' && error.error.trim() !== '') {
        return error.error;
      }

      if (error.status === 0) {
        return 'Backend is unavailable. Start the Electron demo backend and try again.';
      }
    }

    return 'Unable to sign in right now. Try again.';
  }
}