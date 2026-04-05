import { AppService } from './services/app.service';
import {
  Component,
  computed,
  effect,
  inject,
  signal,
  viewChild,
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
import { SearchComponent } from './components/search/search.component';
import { ChooseTableComponent } from './components/choose-table/choose-table.component';
import { EnvironmentComponent } from './components/environment/environment.component';
import { MatButtonModule } from '@angular/material/button';
import { ENVIRONMENT, INTENT } from './enums';
import { UsersComponent } from './components/users/users.component';
import { EnvironmentService } from './services/environment.service';
import { TablesService } from './services/tables.service';
@Component({
  selector: 'app-root',
  standalone: true,
  imports: [
    RouterOutlet,
    MatFormFieldModule,
    MatSelectModule,
    FormsModule,
    ReactiveFormsModule,
    MatInputModule,
    MatCheckboxModule,
    SearchComponent,
    ChooseTableComponent,
    MatButtonModule,
    EnvironmentComponent,
    UsersComponent,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss',
})
export class AppComponent {
  appService = inject(AppService);
  envService = inject(EnvironmentService);
  tableService = inject(TablesService);

  hasResults = computed(
    () =>
      this.tableService.searchResults() &&
      !!Object.keys(this.tableService.searchResults() ?? {}).length,
  );
  moveDataComponent = viewChild(ChooseTableComponent);
  searchDataComponent = viewChild(SearchComponent);
  INTENT = INTENT;
  ENVIRONMENT = ENVIRONMENT;

  handleAction() {
    if (this.appService.intent() === INTENT.MOVE) {
      this.moveDataComponent()?.handleAction();
    } else if (this.appService.intent() === INTENT.SEARCH) {
      this.searchDataComponent()?.handleAction();
    }
  }

  clear() {}
}
