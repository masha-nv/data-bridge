import { ENVIRONMENT, INTENT } from './../../enums';
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
import { EnvironmentService } from '../../services/environment.service';
import { MatRadioModule } from '@angular/material/radio';
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
    MatRadioModule,
  ],
  templateUrl: './environment.component.html',
  styleUrl: './environment.component.scss',
})
export class EnvironmentComponent {
  appService = inject(AppService);
  envService = inject(EnvironmentService);

  intent = input.required<INTENT>();

  enabledEnvSelection = computed(() => this.envService.isEnvSelectionEnabled());

  ENVIRONMENT = ENVIRONMENT;
  INTENT = INTENT;
}
