import { Component, computed, inject, signal, effect } from '@angular/core';
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

@Component({
  selector: 'app-search',
  standalone: true,
  imports: [
    RouterOutlet,
    MatFormFieldModule,
    MatSelectModule,
    FormsModule,
    ReactiveFormsModule,
    MatInputModule,
    MatCheckboxModule,
  ],
  templateUrl: './search.component.html',
  styleUrl: './search.component.scss',
})
export class SearchComponent {
  appService = inject(AppService);

  searchBy = signal('');
  beneId = signal('');
  beneName = signal('');

  constructor() {
    effect(
      () => {
        if (!this.searchBy().includes('beneId')) {
          this.beneId.update(() => '');
        }
        if (!this.searchBy().includes('beneName')) {
          this.beneName.update(() => '');
        }
      },
      { allowSignalWrites: true },
    );
  }
}
