import {
  Component,
  computed,
  effect,
  inject,
  input,
  signal,
} from '@angular/core';
import { RouterOutlet } from '@angular/router';
import {
  FormControl,
  FormGroupDirective,
  NgForm,
  Validators,
  FormsModule,
  ReactiveFormsModule,
} from '@angular/forms';
import { ErrorStateMatcher } from '@angular/material/core';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { AppService } from '../../services/app.service';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-environment',
  standalone: true,
  imports: [
    RouterOutlet,
    MatFormFieldModule,
    MatSelectModule,
    FormsModule,
    ReactiveFormsModule,
    MatInputModule,
    MatCheckboxModule,
    CommonModule,
  ],
  templateUrl: './environment.component.html',
  styleUrl: './environment.component.scss',
})
export class EnvironmentComponent {
  appService = inject(AppService);
  develop = signal(false);
  test = signal(false);
  enabledEnvSelection = this.appService.isEnvSelectionEnabled;
  alignment = input('row');

  constructor() {
    effect(
      () => {
        if (this.appService.intent()) {
          this.appService.enableEnvSelection(false);
        }
      },
      { allowSignalWrites: true },
    );
  }
}
