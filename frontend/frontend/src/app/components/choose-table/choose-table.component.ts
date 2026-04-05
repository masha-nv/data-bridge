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
import { EnvironmentService } from '../../services/environment.service';
import { UsersComponent } from '../users/users.component';
import { ENVIRONMENT } from '../../enums';
import { TablesService } from '../../services/tables.service';

@Component({
  selector: 'app-choose-table',
  standalone: true,
  imports: [
    RouterOutlet,
    MatFormFieldModule,
    MatSelectModule,
    FormsModule,
    ReactiveFormsModule,
    MatInputModule,
    MatCheckboxModule,
    UsersComponent,
  ],
  templateUrl: './choose-table.component.html',
  styleUrl: './choose-table.component.scss',
})
export class ChooseTableComponent implements OnInit {
  appService = inject(AppService);
  tableService = inject(TablesService);
  envService = inject(EnvironmentService);

  tables = signal<string[]>([]);

  ngOnInit(): void {
    this.tableService
      .getTables(ENVIRONMENT.DEVELOP)
      .pipe(tap((response) => this.tables.update(() => response)))
      .subscribe();
  }

  handleAction() {
    this.appService
      .handleAction('move', {
        table: this.tableService.selectedTable(),
        fromEnv: this.envService.fromEnv(),
        toEnv: this.envService.toEnv(),
      })
      .subscribe();
  }

  onTableChange(table: string) {
    this.tableService.handleTableChange(table);
  }
}
