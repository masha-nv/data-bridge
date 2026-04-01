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
import { MoveDataComponent } from './components/move-data/move-data.component';
import { EnvironmentComponent } from './components/environment/environment.component';
import { MatButtonModule } from '@angular/material/button';
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
    MoveDataComponent,
    MatButtonModule,
    EnvironmentComponent,
  ],
  templateUrl: './app.component.html',
  styleUrl: './app.component.scss',
})
export class AppComponent {
  intent = signal('');
  appService = inject(AppService);
  moveDataComponent = viewChild(MoveDataComponent);

  constructor() {
    effect(
      () => {
        if (!!this.intent()) {
          this.appService.triggerIntentChanged(this.intent());
        }
      },
      { allowSignalWrites: true },
    );
  }

  handleAction() {
    if (this.intent() === 'move') {
      this.moveDataComponent()?.handleAction();
    }
  }
}
