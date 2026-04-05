import {
  Component,
  computed,
  inject,
  signal,
  effect,
  OnInit,
} from '@angular/core';
import { RouterOutlet } from '@angular/router';
import {
  FormControl,
  FormGroupDirective,
  NgForm,
  Validators,
  FormsModule,
  ReactiveFormsModule,
  FormBuilder,
  FormGroup,
} from '@angular/forms';
import { ErrorStateMatcher } from '@angular/material/core';
import { MatInputModule } from '@angular/material/input';
import { MatSelectChange, MatSelectModule } from '@angular/material/select';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { AppService } from '../../services/app.service';
import { SearchRequest } from '../../services/interfaces/search-request';
import { EnvironmentService } from '../../services/environment.service';
import { ENVIRONMENT } from '../../enums';
import { UsersComponent } from '../users/users.component';
import { ChooseTableComponent } from '../choose-table/choose-table.component';
import { TablesService } from '../../services/tables.service';

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
    UsersComponent,
    ChooseTableComponent,
  ],
  templateUrl: './search.component.html',
  styleUrl: './search.component.scss',
})
export class SearchComponent implements OnInit {
  appService = inject(AppService);
  envService = inject(EnvironmentService);
  tableService = inject(TablesService);
  fb = inject(FormBuilder);

  form = new FormGroup({});

  ngOnInit(): void {}

  handleSearchBySelection(evt: MatSelectChange) {
    this.tableService.searchBy.set(evt.value);
    for (const f of evt.value) {
      if (!this.form.contains(f)) {
        this.form?.addControl(f, new FormControl(''));
      }
    }
  }

  handleAction() {
    const envs = [
      this.envService.searchEnvDev(),
      this.envService.searchEnvTest(),
    ]
      .map((el, idx) =>
        el ? (idx === 0 ? ENVIRONMENT.DEVELOP : ENVIRONMENT.TEST) : null,
      )
      .filter((en) => !!en) as ENVIRONMENT[];

    const payload: SearchRequest = {
      value: this.form.value,
      envs: envs,
      table: this.tableService.selectedTable(),
    };
    this.appService.handleAction('search', payload).subscribe();
  }
}
