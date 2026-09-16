import json
from pathlib import Path
from datetime import datetime, timedelta
from config import MEMORY_DIR


class ContextManager:
    def __init__(self, data_dir=None):
        if data_dir is None:
            data_dir = MEMORY_DIR
        
        self.data_dir = Path(data_dir)
        self.data_dir.mkdir(exist_ok=True)
        
        # Session courante en mémoire
        self.current_session = None
        self.current_exchanges = []
        self.conversation_history = []
        self.memory_condensed_file = self.data_dir / "memory_condensed.txt"
    
    def start_session(self):
        """Crée une nouvelle session"""
        today = datetime.now().strftime("%Y-%m-%d")
        
        self.current_session = {
            'date': today,
            'start_time': datetime.now().isoformat(),
            'end_time': None,
            'wake_count': 1,
            'exchanges': [],
            'topics': [],
            'sentiment': 'neutral'
        }
        
        self.current_exchanges = []
        self.conversation_history = []
        print(f"📝 Session started: {today}")
    
    def add_exchange(self, user_text, bot_response, duration_seconds=0):
        """Ajoute un échange à la session"""
        if not self.current_session:
            self.start_session()
        
        exchange = {
            'timestamp': datetime.now().isoformat(),
            'user': user_text,
            'bot': bot_response,
            'duration_seconds': duration_seconds
        }
        
        self.current_exchanges.append(exchange)
        self.current_session['exchanges'].append(exchange)
        
        # Garder l'historique pour le LLM
        self.conversation_history.append({
            'role': 'user',
            'content': user_text
        })
        self.conversation_history.append({
            'role': 'assistant',
            'content': bot_response
        })
        
        # Limiter l'historique à 20 messages (10 échanges)
        if len(self.conversation_history) > 20:
            self.conversation_history = self.conversation_history[-20:]
    
    def end_session(self):
        """Sauvegarde la session et retourne en mode repos"""
        if not self.current_session:
            return
        
        self.current_session['end_time'] = datetime.now().isoformat()
        
        # Sauvegarder la session du jour
        today = datetime.now().strftime("%Y-%m-%d")
        session_file = self.data_dir / f"{today}.json"
        
        # Charger les sessions du jour (si existe)
        if session_file.exists():
            try:
                with open(session_file) as f:
                    daily_sessions = json.load(f)
            except:
                daily_sessions = []
        else:
            daily_sessions = []
        
        daily_sessions.append(self.current_session)
        
        # Sauvegarder
        try:
            with open(session_file, 'w') as f:
                json.dump(daily_sessions, f, indent=2)
            print(f"💾 Session saved: {session_file}")
        except Exception as e:
            print(f"❌ Error saving session: {e}")
        
        # Mettre à jour la mémoire condensée
        self.update_condensed_memory()
        
        # Reset
        self.current_session = None
        self.current_exchanges = []
        self.conversation_history = []
    
    def get_context_for_llm(self):
        """Prépare le contexte à envoyer au LLM"""
        context = ""
        
        # Ajouter l'historique complet de la session courante
        if self.current_exchanges:
            context += "=== Current Session ===\n"
            for exchange in self.current_exchanges[-10:]:  # Derniers 10 échanges
                context += f"User: {exchange['user']}\n"
                context += f"Bot: {exchange['bot']}\n\n"
        
        # Ajouter la mémoire condensée (contexte global)
        context += "\n=== Memory (7 days) ===\n"
        context += self.load_condensed_memory()
        
        return context
    
    def load_condensed_memory(self):
        """Charge la mémoire condensée"""
        if self.memory_condensed_file.exists():
            try:
                with open(self.memory_condensed_file) as f:
                    return f.read()
            except:
                return ""
        return ""
    
    def update_condensed_memory(self):
        """Met à jour la mémoire condensée après chaque session"""
        if not self.current_session:
            return
        
        # Charger l'ancienne mémoire
        old_memory = self.load_condensed_memory()
        
        # Générer un résumé de la session
        session_summary = self._summarize_session(self.current_session)
        
        # Combiner: nouvelle session en priorité, puis ancien contexte
        combined = session_summary + "\n" + old_memory
        
        # Tronquer à 1000 chars max
        truncated = combined[:1000]
        
        # Sauvegarder
        try:
            with open(self.memory_condensed_file, 'w') as f:
                f.write(truncated)
            print("💾 Memory condensed updated")
        except Exception as e:
            print(f"❌ Error updating condensed memory: {e}")
    
    def _summarize_session(self, session):
        """Crée un résumé compact d'une session"""
        date = session['date']
        start_time = session['start_time'][:5]  # HH:MM
        exchanges_count = len(session['exchanges'])
        
        # Calculer durée
        try:
            start = datetime.fromisoformat(session['start_time'])
            end = datetime.fromisoformat(session['end_time'])
            duration = int((end - start).total_seconds() / 60)
        except:
            duration = 0
        
        # Topics
        topics = ", ".join(session.get('topics', [])[:3])
        if not topics:
            topics = "general"
        
        # Générer résumé (format compacte)
        summary = f"{date} {start_time} | {exchanges_count}x | {topics} | {duration}min\n"
        
        return summary
    
    def cleanup_old_sessions(self, max_days=7):
        """Supprime les sessions plus anciennes que 7 jours"""
        cutoff_date = datetime.now() - timedelta(days=max_days)
        
        for file in self.data_dir.glob("*.json"):
            # Extraire la date du nom: YYYY-MM-DD.json
            try:
                date_str = file.stem
                file_date = datetime.strptime(date_str, "%Y-%m-%d")
                
                if file_date < cutoff_date:
                    file.unlink()
                    print(f"🗑️  Deleted: {file}")
            except:
                pass
