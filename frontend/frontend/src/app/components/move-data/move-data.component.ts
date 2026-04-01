import {
  Component,
  computed,
  effect,
  inject,
  OnInit,
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
import { tap } from 'rxjs';

@Component({
  selector: 'app-move-data',
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
  templateUrl: './move-data.component.html',
  styleUrl: './move-data.component.scss',
})
export class MoveDataComponent implements OnInit {
  appService = inject(AppService);
  tables = signal<string[]>([]);
  selectedTable = signal('');

  ngOnInit(): void {
    this.appService
      .getTables('develop')
      .pipe(tap((response) => this.tables.update(() => response)))
      .subscribe();
  }

  constructor() {
    effect(
      () => {
        this.appService.enableEnvSelection(!!this.selectedTable());
      },
      { allowSignalWrites: true },
    );
  }

  handleAction() {
    this.appService
      .handleAction('move', {
        table: this.selectedTable(),
        fromEnv: 'test',
        toEnv: 'develop',
      })
      .subscribe();
  }
}
